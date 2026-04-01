package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/anthropics/agentjit/internal/config"
	"github.com/anthropics/agentjit/internal/hooks"
	"github.com/spf13/cobra"
)

var initLocal bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize AgentJIT and install Claude Code hooks",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := config.DefaultPaths()
		if err != nil {
			return err
		}

		// 1. Create directories
		fmt.Println("[AgentJIT] Initializing...")
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
			os.MkdirAll(localSkills, 0755)
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

		// 4. Verify on PATH
		fmt.Println()
		fmt.Println("4. Verifying agentjit is on PATH")
		if path, err := exec.LookPath("agentjit"); err == nil {
			fmt.Printf("   ✓ Found at %s\n", path)
		} else {
			fmt.Println("   ⚠ agentjit not found on PATH. Add it to use hooks.")
		}

		fmt.Println()
		fmt.Println("[AgentJIT] Ready. Hooks will activate on your next Claude Code session.")
		fmt.Println("[AgentJIT] Run 'agentjit bootstrap' to import historical sessions.")
		return nil
	},
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove AgentJIT hooks and optionally delete data",
	RunE: func(cmd *cobra.Command, args []string) error {
		// Uninstall global hooks
		settingsPath, err := config.ClaudeSettingsGlobal()
		if err != nil {
			return err
		}
		if err := hooks.UninstallHooks(settingsPath); err != nil {
			return fmt.Errorf("removing hooks: %w", err)
		}
		fmt.Printf("[AgentJIT] Hooks removed from %s\n", settingsPath)

		paths, err := config.DefaultPaths()
		if err != nil {
			return err
		}

		fmt.Printf("[AgentJIT] Data directory at %s was not removed. Delete manually if desired.\n", paths.Root)
		return nil
	},
}

func init() {
	initCmd.Flags().BoolVar(&initLocal, "local", false, "Install hooks into project-local .claude/settings.json")
	initCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(initCmd)
}
