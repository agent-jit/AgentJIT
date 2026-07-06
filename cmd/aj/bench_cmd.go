package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"

	"github.com/agent-jit/agentjit/internal/bench"
	"github.com/agent-jit/agentjit/internal/config"
	"github.com/spf13/cobra"
)

var (
	benchTasksFile   string
	benchArm         string
	benchRollouts    int
	benchModel       string
	benchJSON        bool
	benchDryRun      bool
	benchCompare     bool
	benchCompileCost int
)

var benchCmd = &cobra.Command{
	Use:   "bench",
	Short: "Benchmark AgentJIT skill ROI (baseline vs JIT) on a task suite",
	Long: "Runs each task in a JSONL suite for N rollouts under an arm and reports\n" +
		"tokens-to-success at iso-accuracy — only verified rollouts count.\n" +
		"Point AJ at an isolated sandbox with AJ_HOME so real data is untouched.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if benchTasksFile == "" {
			return fmt.Errorf("--tasks is required (a JSONL task file)")
		}
		arm := bench.Arm(benchArm)
		if arm != bench.ArmBaseline && arm != bench.ArmJIT {
			return fmt.Errorf("--arm must be %q or %q", bench.ArmBaseline, bench.ArmJIT)
		}
		if benchRollouts < 1 {
			return fmt.Errorf("--rollouts must be >= 1")
		}

		tasks, err := bench.LoadTasks(benchTasksFile)
		if err != nil {
			return fmt.Errorf("loading tasks: %w", err)
		}
		if len(tasks) == 0 {
			fmt.Println("[AJ] No tasks in suite.")
			return nil
		}

		var agent bench.AgentRunner = bench.ClaudeRunner{ExtraArgs: modelArgs(benchModel)}
		if benchDryRun {
			agent = dryRunAgent{}
		}
		runner := bench.Runner{Agent: agent, Verifier: bench.CommandVerifier{}}

		if benchCompare {
			return runCompare(runner, tasks)
		}

		results := make([]bench.TaskResult, 0, len(tasks))
		for _, task := range tasks {
			if !benchJSON {
				fmt.Printf("[AJ] %s (%s) x%d ... ", task.ID, arm, benchRollouts)
			}
			res := runner.RunTask(context.Background(), task, arm, benchRollouts)
			results = append(results, res)
			if !benchJSON {
				printTaskResult(res)
			}
		}

		if benchJSON {
			enc := json.NewEncoder(os.Stdout)
			enc.SetIndent("", "  ")
			return enc.Encode(results)
		}
		return nil
	},
}

// runCompare runs every task under both arms and reports the baseline-vs-JIT
// comparison, including break-even when a compile cost is available.
func runCompare(runner bench.Runner, tasks []bench.Task) error {
	compileCost := benchCompileCost
	// When not given explicitly, read the real compile cost from the sandbox's
	// stats (the tokens AgentJIT spent building the skills under test).
	if compileCost == 0 {
		if paths, err := config.DefaultPaths(); err == nil {
			if c, err := bench.CompileCostFromStats(paths.Stats); err == nil && c > 0 {
				compileCost = c
				if !benchJSON {
					fmt.Printf("[AJ] Using compile cost %d tokens from %s\n", compileCost, paths.Stats)
				}
			}
		}
	}

	comparisons := make([]bench.Comparison, 0, len(tasks))
	for _, task := range tasks {
		baseline := runner.RunTask(context.Background(), task, bench.ArmBaseline, benchRollouts)
		jit := runner.RunTask(context.Background(), task, bench.ArmJIT, benchRollouts)
		c := bench.Compare(baseline, jit)
		comparisons = append(comparisons, c)
		if !benchJSON {
			printComparison(c, compileCost)
		}
	}
	if benchJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(comparisons)
	}
	return nil
}

func printComparison(c bench.Comparison, compileCost int) {
	fmt.Printf("[AJ] %s", c.Task)
	if c.Shape != "" {
		fmt.Printf(" [%s]", c.Shape)
	}
	fmt.Printf(": baseline %.0f%% / jit %.0f%%", c.BaselineSuccess*100, c.JITSuccess*100)
	if !c.Comparable {
		fmt.Println(" — not comparable (an arm had no verified rollout)")
		return
	}
	if !c.IsoAccuracy {
		fmt.Print(" — WARN: success differs, token comparison not iso-accuracy")
	}
	fmt.Printf("\n     T2S baseline %.0f (med %.0f) vs jit %.0f (med %.0f), saving %.0f/use",
		c.Baseline.Mean, c.Baseline.Median, c.JIT.Mean, c.JIT.Median, c.PerInvocationSaving)
	if compileCost > 0 {
		if be, ok := c.BreakEven(compileCost); ok {
			fmt.Printf(", break-even @ %.1f invocations", be)
		} else {
			fmt.Print(", never breaks even")
		}
	}
	fmt.Println()
}

// modelArgs returns claude CLI args for a fixed model, or nil to use the default.
func modelArgs(model string) []string {
	if model == "" {
		return nil
	}
	return []string{"--model", model}
}

func printTaskResult(res bench.TaskResult) {
	mean, ok := res.MeanTokensToSuccess()
	if ok {
		fmt.Printf("success %.0f%%, T2S %.0f tokens\n", res.SuccessRate()*100, mean)
	} else {
		fmt.Printf("success %.0f%% (no verified rollout — no T2S)\n", res.SuccessRate()*100)
	}
}

// dryRunAgent is a placeholder agent used by --dry-run to exercise the harness
// wiring without invoking claude. It reports zero usage.
type dryRunAgent struct{}

func (dryRunAgent) Run(context.Context, bench.Task) (bench.Usage, error) {
	return bench.Usage{}, nil
}

func init() {
	benchCmd.Flags().StringVar(&benchTasksFile, "tasks", "", "Path to a JSONL task suite")
	benchCmd.Flags().StringVar(&benchArm, "arm", string(bench.ArmBaseline), "Arm to run: baseline or jit")
	benchCmd.Flags().IntVar(&benchRollouts, "rollouts", 3, "Rollouts per task (averaged)")
	benchCmd.Flags().StringVar(&benchModel, "model", "", "Pin the claude model (e.g. claude-opus-4-8)")
	benchCmd.Flags().BoolVar(&benchJSON, "json", false, "Emit per-task results as JSON")
	benchCmd.Flags().BoolVar(&benchDryRun, "dry-run", false, "Exercise the harness without invoking claude")
	benchCmd.Flags().BoolVar(&benchCompare, "compare", false, "Run both arms per task and report baseline-vs-JIT")
	benchCmd.Flags().IntVar(&benchCompileCost, "compile-cost", 0, "Skill compile cost (tokens) for break-even; auto-read from AJ_HOME stats if unset")
	rootCmd.AddCommand(benchCmd)
}
