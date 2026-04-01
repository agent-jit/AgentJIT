package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

var configAll bool

var configCmd = &cobra.Command{
	Use:   "config",
	Short: "View or modify AgentJIT configuration",
}

var configGetCmd = &cobra.Command{
	Use:   "get [key]",
	Short: "Get a config value",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("[AgentJIT] config get not yet implemented")
		return nil
	},
}

var configSetCmd = &cobra.Command{
	Use:   "set <key> <value>",
	Short: "Set a config value",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("[AgentJIT] config set not yet implemented")
		return nil
	},
}

var configResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset configuration to defaults",
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("[AgentJIT] config reset not yet implemented")
		return nil
	},
}

func init() {
	configGetCmd.Flags().BoolVar(&configAll, "all", false, "Dump full config")
	configCmd.AddCommand(configGetCmd, configSetCmd, configResetCmd)
	rootCmd.AddCommand(configCmd)
}
