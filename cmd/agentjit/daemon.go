package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var daemonCmd = &cobra.Command{
	Use:   "daemon",
	Short: "Manage the AgentJIT daemon",
}

var ifNotRunning bool

var daemonStartCmd = &cobra.Command{
	Use:   "start",
	Short: "Start the AgentJIT daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("[AgentJIT] daemon start not yet implemented")
		return nil
	},
}

var daemonStopCmd = &cobra.Command{
	Use:   "stop",
	Short: "Stop the AgentJIT daemon",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("[AgentJIT] daemon stop not yet implemented")
		return nil
	},
}

var daemonStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show daemon status",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("[AgentJIT] daemon status not yet implemented")
		return nil
	},
}

func init() {
	daemonStartCmd.Flags().BoolVar(&ifNotRunning, "if-not-running", false, "Start only if not already running")
	daemonCmd.AddCommand(daemonStartCmd, daemonStopCmd, daemonStatusCmd)
	rootCmd.AddCommand(daemonCmd)
}
