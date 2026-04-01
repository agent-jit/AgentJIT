package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var ingestCmd = &cobra.Command{
	Use:    "ingest",
	Short:  "Receive hook JSON on stdin and forward to daemon",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("[AgentJIT] ingest not yet implemented")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(ingestCmd)
}
