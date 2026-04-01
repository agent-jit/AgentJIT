package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var skillsCmd = &cobra.Command{
	Use:   "skills",
	Short: "Manage generated skills",
}

var skillsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List generated skills with ROI metrics",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("[AgentJIT] skills list not yet implemented")
		return nil
	},
}

var skillsRemoveCmd = &cobra.Command{
	Use:   "remove <name>",
	Short: "Remove a generated skill and deregister it",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("[AgentJIT] skills remove not yet implemented")
		return nil
	},
}

func init() {
	skillsCmd.AddCommand(skillsListCmd, skillsRemoveCmd)
	rootCmd.AddCommand(skillsCmd)
}
