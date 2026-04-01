package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var dreamCmd = &cobra.Command{
	Use:   "dream",
	Short: "Trigger the JIT compilation/reflection phase",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("[AgentJIT] dream not yet implemented")
		return nil
	},
}

func init() {
	rootCmd.AddCommand(dreamCmd)
}
