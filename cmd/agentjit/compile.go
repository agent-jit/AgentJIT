package main

import (
	"fmt"

	"github.com/anthropics/agentjit/internal/compile"
	"github.com/anthropics/agentjit/internal/config"
	"github.com/anthropics/agentjit/prompts"
	"github.com/spf13/cobra"
)

var compileCmd = &cobra.Command{
	Use:   "compile",
	Short: "Trigger the JIT compilation phase",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := config.DefaultPaths()
		if err != nil {
			return err
		}

		cfg, err := config.Load(paths.Config)
		if err != nil {
			return fmt.Errorf("loading config: %w", err)
		}

		return compile.RunCompile(paths, cfg, prompts.Compiler)
	},
}

func init() {
	rootCmd.AddCommand(compileCmd)
}
