package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/agent-jit/agentjit/internal/config"
	"github.com/agent-jit/agentjit/internal/hooks"
	"github.com/agent-jit/agentjit/internal/skills"
	"github.com/spf13/cobra"
)

var initLocal bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize AJ and install Claude Code hooks",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := config.DefaultPaths()
		if err != nil {
			return err
		}

		// 1. Create directories
		fmt.Println("[AJ] Initializing...")
		fmt.Println()
		fmt.Println("1. Creating directories")
		if err := paths.EnsureDirs(); err != nil {
			return fmt.Errorf("creating directories: %w", err)
		}
		fmt.Printf("   ✓ %s\n", paths.Root)
		fmt.Printf("   ✓ %s\n", paths.Logs)
		fmt.Printf("   ✓ %s\n", paths.Skills)

		// 2. Write default config (if not exists)
		fmt.Println()
		fmt.Println("2. Writing default config")
		if _, err := os.Stat(paths.Config); os.IsNotExist(err) {
			if err := config.Save(paths.Config, config.DefaultConfig()); err != nil {
				return fmt.Errorf("writing config: %w", err)
			}
			fmt.Printf("   ✓ %s\n", paths.Config)
		} else {
			fmt.Printf("   ✓ %s (already exists)\n", paths.Config)
		}

		// 3. Install hooks
		fmt.Println()
		fmt.Println("3. Installing Claude Code hooks")
		var settingsPath string
		if initLocal {
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			settingsPath = config.ClaudeSettingsLocal(cwd)
			// Also create local skills directory
			localSkills := filepath.Join(cwd, ".claude", "skills")
			if err := os.MkdirAll(localSkills, 0755); err != nil {
				return fmt.Errorf("creating local skills dir: %w", err)
			}
		} else {
			settingsPath, err = config.ClaudeSettingsGlobal()
			if err != nil {
				return err
			}
		}

		if err := hooks.InstallHooks(settingsPath); err != nil {
			return fmt.Errorf("installing hooks: %w", err)
		}
		fmt.Printf("   ✓ Hooks installed in %s\n", settingsPath)

		// 4. Sync skill symlinks
		fmt.Println()
		fmt.Println("4. Syncing skills to Claude Code")
		claudeSkillsDir, csErr := config.ClaudeSkillsGlobal()
		if csErr == nil {
			if err := skills.SyncLinks(paths.Skills, claudeSkillsDir); err != nil {
				fmt.Printf("   ⚠ Could not sync skills: %v\n", err)
			} else {
				existing, _ := os.ReadDir(paths.Skills)
				count := 0
				for _, e := range existing {
					if e.IsDir() {
						count++
					}
				}
				fmt.Printf("   ✓ %d skills linked to %s\n", count, claudeSkillsDir)
			}
		} else {
			fmt.Printf("   ⚠ Could not determine Claude skills directory: %v\n", csErr)
		}

		// 5. Verify on PATH
		fmt.Println()
		fmt.Println("5. Verifying aj is on PATH")
		if path, err := exec.LookPath("aj"); err == nil {
			fmt.Printf("   ✓ Found at %s\n", path)
		} else {
			fmt.Println("   ⚠ aj not found on PATH. Add it to use hooks.")
		}

		fmt.Println()
		fmt.Println("[AJ] Ready. Hooks will activate on your next Claude Code session.")
		fmt.Println("[AJ] Run 'aj bootstrap' to import historical sessions.")
		return nil
	},
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove AJ hooks and optionally delete data",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Uninstall global hooks
		settingsPath, err := config.ClaudeSettingsGlobal()
		if err != nil {
			return err
		}
		if err := hooks.UninstallHooks(settingsPath); err != nil {
			return fmt.Errorf("removing hooks: %w", err)
		}
		fmt.Printf("[AJ] Hooks removed from %s\n", settingsPath)

		paths, err := config.DefaultPaths()
		if err != nil {
			return err
		}

		fmt.Printf("[AJ] Data directory at %s was not removed. Delete manually if desired.\n", paths.Root)
		return nil
	},
}

func init() {
	initCmd.Flags().BoolVar(&initLocal, "local", false, "Install hooks into project-local .claude/settings.json")
	initCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(initCmd)
}
