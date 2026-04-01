package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var bootstrapSince string
var bootstrapProject string
var bootstrapDryRun bool

var bootstrapCmd = &cobra.Command{
	Use:   "bootstrap",
	Short: "Import historical Claude Code transcripts into logs",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("[AgentJIT] bootstrap not yet implemented")
		return nil
	},
}

func init() {
	bootstrapCmd.Flags().StringVar(&bootstrapSince, "since", "", "Only transcripts after this date (YYYY-MM-DD)")
	bootstrapCmd.Flags().StringVar(&bootstrapProject, "project", "", "Only transcripts for this project path")
	bootstrapCmd.Flags().BoolVar(&bootstrapDryRun, "dry-run", false, "Show what would be processed without writing")
	rootCmd.AddCommand(bootstrapCmd)
}
