package main

import (
	"fmt"
	"sort"

	"github.com/agent-jit/agentjit/internal/compile"
	"github.com/agent-jit/agentjit/internal/config"
	"github.com/agent-jit/agentjit/internal/ingest"
	"github.com/agent-jit/agentjit/internal/trace"
	"github.com/agent-jit/agentjit/internal/tui"
	"github.com/spf13/cobra"
)

var traceAll bool
var traceTools []string
var traceMinWeight int
var traceMinLength int
var traceMinFreq int

// loadTraceData loads events and builds a trace graph. Shared by trace subcommands.
func loadTraceData(all bool) ([]ingest.Event, *trace.TraceGraph, error) {
	paths, err := config.DefaultPaths()
	if err != nil {
		return nil, nil, err
	}

	cfg, err := config.Load(paths.Config)
	if err != nil {
		return nil, nil, fmt.Errorf("loading config: %w", err)
	}

	if all {
		fmt.Print("[AJ] Loading ALL events under retention... ")
	} else {
		fmt.Print("[AJ] Loading events... ")
	}

	var events []ingest.Event
	if all {
		events, err = compile.GatherAllLogs(paths, cfg.Compile.MaxContextLines)
	} else {
		events, err = compile.GatherUnprocessedLogs(paths, cfg.Compile.MaxContextLines)
	}
	if err != nil {
		return nil, nil, fmt.Errorf("gathering events: %w", err)
	}
	fmt.Printf("%d events\n", len(events))

	if len(events) == 0 {
		return nil, nil, nil
	}

	fmt.Print("[AJ] Building trace graph... ")
	g := trace.BuildGraph(events)
	fmt.Printf("%d nodes, %d edges\n", len(g.Nodes), countEdges(g))

	return events, g, nil
}

var traceCmd = &cobra.Command{
	Use:   "trace",
	Short: "Explore the trace graph of tool-call patterns",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := config.DefaultPaths()
		if err != nil {
			return err
		}
		cfg, err := config.Load(paths.Config)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		events, g, err := loadTraceData(traceAll)
		if err != nil {
			return err
		}
		if events == nil {
			fmt.Println("[AJ] No events to analyze. Run some Claude Code sessions first.")
			return nil
		}

		g = filterGraph(g, traceTools, 0)
		if len(g.Nodes) == 0 {
			fmt.Println("[AJ] No nodes match the filter criteria.")
			return nil
		}

		fmt.Print("[AJ] Detecting hot paths... ")
		minFreq := cfg.Compile.MinPatternFrequency
		if traceMinFreq > 0 {
			minFreq = traceMinFreq
		}
		trace.DetectDataFlowEdges(g, events, 5)
		hotPaths := trace.FindHotPaths(g, minFreq, traceMinLength, 20)
		fmt.Printf("%d found\n", len(hotPaths))

		if len(hotPaths) == 0 {
			fmt.Println("[AJ] No hot paths above threshold. Lower min_pattern_frequency or gather more sessions.")
			return nil
		}

		// Filter hot paths by min-weight (minimum frequency).
		if traceMinWeight > 1 {
			filtered := hotPaths[:0]
			for _, hp := range hotPaths {
				if hp.Frequency >= traceMinWeight {
					filtered = append(filtered, hp)
				}
			}
			hotPaths = filtered
		}

		if len(hotPaths) == 0 {
			fmt.Println("[AJ] No hot paths meet the minimum weight threshold.")
			return nil
		}

		// Rank first, then cap to a display limit before the expensive parameterize step.
		ranked := trace.RankHotPaths(hotPaths)
		const maxDisplay = 100
		if len(ranked) > maxDisplay {
			ranked = ranked[:maxDisplay]
		}
		// Rebuild hotPaths from the ranked (capped) list.
		hotPaths = make([]trace.HotPath, len(ranked))
		for i, r := range ranked {
			hotPaths[i] = r.HotPath
		}

		fmt.Print("[AJ] Parameterizing top paths... ")
		patterns := trace.Parameterize(hotPaths, events, g)
		fmt.Printf("%d done\n", len(patterns))
		annotated := make([]tui.AnnotatedPath, len(hotPaths))
		for i, hp := range hotPaths {
			labels := make([]string, len(hp.NodeIDs))
			for j, id := range hp.NodeIDs {
				node := g.Nodes[id]
				if node.ToolName == "Bash" {
					if cmd, ok := node.InputShape["command"]; ok {
						// Truncate long commands
						if len(cmd) > 50 {
							cmd = cmd[:47] + "..."
						}
						labels[j] = cmd
					} else {
						labels[j] = node.ToolName
					}
				} else {
					labels[j] = node.ToolName
				}
			}

			var pattern *trace.Pattern
			if i < len(patterns) {
				p := patterns[i]
				pattern = &p
			}

			// Count data-flow edges and build annotations
			dfCount := 0
			var dfAnnotations []tui.DataFlowAnnotation
			for j := 0; j < len(hp.NodeIDs)-1; j++ {
				fromID := hp.NodeIDs[j]
				toID := hp.NodeIDs[j+1]
				if adj, ok := g.Edges[fromID]; ok {
					if edge, ok := adj[toID]; ok && edge.DataFlow {
						dfCount++
						dfAnnotations = append(dfAnnotations, tui.DataFlowAnnotation{
							FromStep:   j,
							ToStep:     j + 1,
							FlowTokens: edge.FlowTokens,
						})
					}
				}
			}

			conf := trace.ScorePatternWithDataFlow(patterns[i], dfCount)
			savings := len(patterns[i].Steps) * 200 // rough estimate

			var compilationValue int
			compilationValue = ranked[i].CompilationValue

			annotated[i] = tui.AnnotatedPath{
				Path:             hp,
				Pattern:          pattern,
				Labels:           labels,
				Confidence:       conf,
				Savings:          savings,
				CompilationValue: compilationValue,
				DataFlowEdges:    dfAnnotations,
			}
		}

		// Sort by score descending.
		sort.Slice(annotated, func(i, j int) bool {
			return annotated[i].Path.Score > annotated[j].Path.Score
		})

		return tui.Run(annotated, g)
	},
}

func countEdges(g *trace.TraceGraph) int {
	count := 0
	for _, adj := range g.Edges {
		count += len(adj)
	}
	return count
}

func init() {
	traceCmd.Flags().BoolVar(&traceAll, "all", false, "Analyze all events under retention (ignore compile marker)")
	traceCmd.Flags().StringSliceVar(&traceTools, "tool", nil, "Filter by tool name (e.g. --tool Bash,Read)")
	traceCmd.Flags().IntVar(&traceMinWeight, "min-weight", 1, "Minimum edge weight to include")
	traceCmd.Flags().IntVar(&traceMinLength, "min-length", 2, "Minimum path length (number of nodes)")
	traceCmd.Flags().IntVar(&traceMinFreq, "min-freq", 0, "Minimum session frequency (default: from config, usually 3)")
	rootCmd.AddCommand(traceCmd)
}
