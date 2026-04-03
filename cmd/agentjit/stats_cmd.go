package main

import (
	"fmt"
	"os"

	"github.com/anthropics/agentjit/internal/config"
	"github.com/anthropics/agentjit/internal/stats"
	"github.com/spf13/cobra"
)

var statsJSON bool

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show token usage statistics and ROI",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := config.DefaultPaths()
		if err != nil {
			return err
		}
		return stats.PrintStats(paths.Stats, statsJSON)
	},
}

var statsResetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Clear all recorded statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := config.DefaultPaths()
		if err != nil {
			return err
		}

		if err := os.Remove(paths.Stats); err != nil && !os.IsNotExist(err) {
			return err
		}

		fmt.Println("[AJ] Stats reset.")
		return nil
	},
}

func init() {
	statsCmd.Flags().BoolVar(&statsJSON, "json", false, "Output stats as JSON")
	statsCmd.AddCommand(statsResetCmd)
	rootCmd.AddCommand(statsCmd)
}
