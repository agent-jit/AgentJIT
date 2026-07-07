package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

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
	benchGen         string
	benchN           []int
	benchWorkdir     string
)

var benchCmd = &cobra.Command{
	Use:   "bench",
	Short: "Benchmark AgentJIT skill ROI (baseline vs JIT) on a task suite",
	Long: "Runs tasks for N rollouts under an arm and reports tokens-to-success at\n" +
		"iso-accuracy — only verified rollouts count. Load tasks from a JSONL suite\n" +
		"(--tasks) or generate a repetition fixture (--gen <shape> --n 1,2,4).\n" +
		"Point AJ at an isolated sandbox with AJ_HOME so real data is untouched.",
	RunE: func(cmd *cobra.Command, args []string) error {
		if benchTasksFile == "" && benchGen == "" {
			return fmt.Errorf("provide --tasks <file> or --gen <shape>")
		}
		if benchTasksFile != "" && benchGen != "" {
			return fmt.Errorf("--tasks and --gen are mutually exclusive")
		}
		arm := bench.Arm(benchArm)
		if arm != bench.ArmBaseline && arm != bench.ArmJIT {
			return fmt.Errorf("--arm must be %q or %q", bench.ArmBaseline, bench.ArmJIT)
		}
		if benchRollouts < 1 {
			return fmt.Errorf("--rollouts must be >= 1")
		}

		var agent bench.AgentRunner = bench.ClaudeRunner{ExtraArgs: modelArgs(benchModel)}
		if benchDryRun {
			agent = dryRunAgent{}
		}
		runner := bench.Runner{Agent: agent, Verifier: bench.CommandVerifier{}}

		// --gen --compare regenerates a fresh fixture per arm (baseline mutates
		// the tree, so the JIT arm must start clean) and installs the skill for
		// the JIT arm. This is the only path that yields a real break-even.
		if benchGen != "" && benchCompare {
			return runGenCompare(runner, benchGen, benchN, benchWorkdir)
		}

		var tasks []bench.Task
		var err error
		if benchGen != "" {
			tasks, err = generateTasks(benchGen, benchN, benchWorkdir)
		} else {
			tasks, err = bench.LoadTasks(benchTasksFile)
		}
		if err != nil {
			return err
		}
		if len(tasks) == 0 {
			fmt.Println("[AJ] No tasks in suite.")
			return nil
		}

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

// runGenCompare sweeps the repeat counts, regenerating a FRESH fixture for each
// arm (baseline mutates the tree in place, so the JIT arm must not inherit it)
// and giving the JIT arm the workflow skill. Reports break-even per count.
//
// The JIT arm's skill comes from the real `aj compile` when the fixture is a
// CompilableFixture (seed logs -> compile -> generated skill), otherwise from a
// hand-written SkilledFixture skill. Compile cost defaults to what aj compile
// actually spent (0 on the deterministic path) unless --compile-cost overrides.
func runGenCompare(runner bench.Runner, shape string, counts []int, workdir string) error {
	fixture, ok := bench.FixtureByShape(shape)
	if !ok {
		return fmt.Errorf("unknown --gen shape %q (have: %v)", shape, bench.FixtureShapes())
	}
	compilable, canCompile := fixture.(bench.CompilableFixture)
	skilled, canSkill := fixture.(bench.SkilledFixture)
	if !canCompile && !canSkill {
		return fmt.Errorf("fixture %q has no skill for the JIT arm; --gen --compare needs one", shape)
	}
	if len(counts) == 0 {
		counts = []int{3}
	}
	base := workdir
	if base == "" {
		dir, err := os.MkdirTemp("", "aj-bench-")
		if err != nil {
			return err
		}
		base = dir
		fmt.Printf("[AJ] Generated fixtures under %s\n", base)
	}

	// realCompileCost is captured by the JIT setup when using aj compile.
	realCompileCost := 0
	ajBin, _ := os.Executable() // this binary is `aj`

	// The JIT arm gets the workflow skill before its episode: real compile when
	// available (and record its true token cost), else the hand-written skill.
	runner.Setup = func(task bench.Task, arm bench.Arm) error {
		if arm != bench.ArmJIT {
			return nil
		}
		if canCompile {
			cost, err := bench.CompileSkill(context.Background(), ajBin, compilable, task)
			if err != nil {
				return err
			}
			realCompileCost = cost
			return nil
		}
		return skilled.InstallSkill(task)
	}

	comparisons := make([]bench.Comparison, 0, len(counts))
	for _, n := range counts {
		// Fresh, separate trees per arm so neither sees the other's edits.
		baseTask, err := genInto(fixture, base, fmt.Sprintf("baseline-%s-%d", shape, n), n)
		if err != nil {
			return err
		}
		jitTask, err := genInto(fixture, base, fmt.Sprintf("jit-%s-%d", shape, n), n)
		if err != nil {
			return err
		}
		baseline := runner.RunTask(context.Background(), baseTask, bench.ArmBaseline, benchRollouts)
		jit := runner.RunTask(context.Background(), jitTask, bench.ArmJIT, benchRollouts)
		// Compare wants matching task IDs; both were generated at the same count.
		jit.Task = baseline.Task
		c := bench.Compare(baseline, jit)
		comparisons = append(comparisons, c)
		// Break-even uses an explicit --compile-cost, else the cost aj compile
		// actually spent (captured by the JIT setup; 0 on the deterministic path).
		cost := benchCompileCost
		if cost == 0 {
			cost = realCompileCost
		}
		if !benchJSON {
			printComparison(c, cost)
		}
	}
	if benchJSON {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(comparisons)
	}
	return nil
}

// genInto generates a fixture into base/sub and returns its Task.
func genInto(fixture bench.Fixture, base, sub string, n int) (bench.Task, error) {
	dir := filepath.Join(base, sub)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return bench.Task{}, err
	}
	return fixture.Generate(dir, n)
}

// generateTasks materializes a repetition fixture at each requested repeat count
// under workdir (a temp dir when empty), returning one Task per count so a
// benchmark can sweep the break-even curve.
func generateTasks(shape string, counts []int, workdir string) ([]bench.Task, error) {
	fixture, ok := bench.FixtureByShape(shape)
	if !ok {
		return nil, fmt.Errorf("unknown --gen shape %q (have: %v)", shape, bench.FixtureShapes())
	}
	if len(counts) == 0 {
		counts = []int{3}
	}
	if workdir == "" {
		dir, err := os.MkdirTemp("", "aj-bench-")
		if err != nil {
			return nil, err
		}
		workdir = dir
		fmt.Printf("[AJ] Generated fixtures under %s\n", workdir)
	}

	tasks := make([]bench.Task, 0, len(counts))
	for _, n := range counts {
		// Each count gets its own subdir so fixtures never collide.
		dir := filepath.Join(workdir, fmt.Sprintf("%s-%d", shape, n))
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return nil, err
		}
		task, err := fixture.Generate(dir, n)
		if err != nil {
			return nil, fmt.Errorf("generate %s n=%d: %w", shape, n, err)
		}
		tasks = append(tasks, task)
	}
	return tasks, nil
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
	benchCmd.Flags().StringVar(&benchGen, "gen", "", "Generate a repetition fixture by shape instead of --tasks (e.g. nullcheck)")
	benchCmd.Flags().IntSliceVar(&benchN, "n", nil, "Repeat counts for --gen (e.g. --n 1,2,4 sweeps the curve)")
	benchCmd.Flags().StringVar(&benchWorkdir, "workdir", "", "Where --gen writes fixtures (default: a temp dir)")
	rootCmd.AddCommand(benchCmd)
}
