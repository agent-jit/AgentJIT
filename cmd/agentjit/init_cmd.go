package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var initLocal bool

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize AgentJIT and install Claude Code hooks",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("[AgentJIT] init not yet implemented")
		return nil
	},
}

var uninstallCmd = &cobra.Command{
	Use:   "uninstall",
	Short: "Remove AgentJIT hooks and optionally delete data",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("[AgentJIT] uninstall not yet implemented")
		return nil
	},
}

func init() {
	initCmd.Flags().BoolVar(&initLocal, "local", false, "Install hooks into project-local .claude/settings.json")
	initCmd.AddCommand(uninstallCmd)
	rootCmd.AddCommand(initCmd)
}
