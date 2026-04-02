package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/anthropics/agentjit/internal/config"
	"github.com/anthropics/agentjit/internal/dream"
	"github.com/spf13/cobra"
)

var dreamCmd = &cobra.Command{
	Use:   "dream",
	Short: "Trigger the JIT compilation/reflection phase",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := config.DefaultPaths()
		if err != nil {
			return err
		}

		cfg, err := config.Load(paths.Config)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		// Find the compiler prompt
		exe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("finding executable: %w", err)
		}
		promptPath := filepath.Join(filepath.Dir(exe), "prompts", "compiler.md")

		// Fallback: check relative to working directory
		if _, err := os.Stat(promptPath); os.IsNotExist(err) {
			cwd, _ := os.Getwd()
			promptPath = filepath.Join(cwd, "prompts", "compiler.md")
		}

		if _, err := os.Stat(promptPath); os.IsNotExist(err) {
			return fmt.Errorf("compiler prompt not found at %s — run from the agentjit project directory or install properly", promptPath)
		}

		return dream.RunDream(paths, cfg, promptPath)
	},
}

func init() {
	rootCmd.AddCommand(dreamCmd)
}
