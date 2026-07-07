package bench

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/agent-jit/agentjit/internal/config"
)

// CompileSkill runs the real `aj compile` over a CompilableFixture's seeded
// session logs in an isolated AJ_HOME, then makes the generated skill available
// to the task's episode (copied into task.RepoDir/.claude/skills). It returns
// the compile cost in tokens (0 for the deterministic path) read from the
// sandbox stats.
//
// ajBin is the path to the aj binary to invoke (so the benchmark exercises the
// real compiler, not a reimplementation).
func CompileSkill(ctx context.Context, ajBin string, fx CompilableFixture, task Task) (int, error) {
	ajHome, err := os.MkdirTemp("", "aj-compile-")
	if err != nil {
		return 0, err
	}
	paths := config.PathsFromRoot(ajHome)
	if err := paths.EnsureDirs(); err != nil {
		return 0, err
	}

	// 1. Seed the workflow's session logs so there's a pattern to detect.
	if err := fx.SeedSessions(paths.Logs); err != nil {
		return 0, fmt.Errorf("seeding sessions: %w", err)
	}

	// 2. Run the real aj compile against that sandbox.
	cmd := exec.CommandContext(ctx, ajBin, "compile")
	cmd.Env = append(os.Environ(), "AJ_HOME="+ajHome)
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	if err := cmd.Run(); err != nil {
		return 0, fmt.Errorf("aj compile: %w\n%s", err, out.String())
	}

	// 3. Locate the generated skill and copy it into the task's project skills.
	srcSkill := filepath.Join(paths.Skills, fx.SkillName())
	if _, err := os.Stat(srcSkill); err != nil {
		return 0, fmt.Errorf("aj compile produced no skill %q under %s:\n%s", fx.SkillName(), paths.Skills, out.String())
	}
	dstSkill := filepath.Join(task.RepoDir, ".claude", "skills", fx.SkillName())
	if err := copyTree(srcSkill, dstSkill); err != nil {
		return 0, fmt.Errorf("installing compiled skill: %w", err)
	}

	// 4. Read the real compile cost (0 for the zero-token deterministic path).
	cost, err := CompileCostFromStats(paths.Stats)
	if err != nil {
		return 0, err
	}
	return cost, nil
}

// copyTree recursively copies a directory tree.
func copyTree(src, dst string) error {
	return filepath.Walk(src, func(p string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, p)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if info.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
			return err
		}
		return os.WriteFile(target, data, info.Mode())
	})
}
