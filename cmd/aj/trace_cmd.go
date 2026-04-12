package main

import (
	"fmt"

	"github.com/agent-jit/agentjit/internal/compile"
	"github.com/agent-jit/agentjit/internal/config"
	"github.com/agent-jit/agentjit/internal/trace"
	"github.com/agent-jit/agentjit/internal/tui"
	"github.com/spf13/cobra"
)

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

		fmt.Print("[AJ] Loading events... ")
		events, err := compile.GatherUnprocessedLogs(paths, cfg.Compile.MaxContextLines)
		if err != nil {
			return fmt.Errorf("gathering events: %w", err)
		}
		fmt.Printf("%d events\n", len(events))

		if len(events) == 0 {
			fmt.Println("[AJ] No events to analyze. Run some Claude Code sessions first.")
			return nil
		}

		fmt.Print("[AJ] Building trace graph... ")
		g := trace.BuildGraph(events)
		fmt.Printf("%d nodes, %d edges\n", len(g.Nodes), countEdges(g))

		fmt.Print("[AJ] Detecting hot paths... ")
		hotPaths := trace.FindHotPaths(g, cfg.Compile.MinPatternFrequency, 2, 20)
		fmt.Printf("%d found\n", len(hotPaths))

		if len(hotPaths) == 0 {
			fmt.Println("[AJ] No hot paths above threshold. Lower min_pattern_frequency or gather more sessions.")
			return nil
		}

		// Parameterize and annotate
		patterns := trace.Parameterize(hotPaths, events, g)
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

			conf := trace.ScorePattern(patterns[i])
			savings := len(patterns[i].Steps) * 200 // rough estimate

			annotated[i] = tui.AnnotatedPath{
				Path:       hp,
				Pattern:    pattern,
				Labels:     labels,
				Confidence: conf,
				Savings:    savings,
			}
		}

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
	rootCmd.AddCommand(traceCmd)
}
