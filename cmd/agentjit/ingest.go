package main

import (
	"os"

	"github.com/anthropics/agentjit/internal/config"
	"github.com/anthropics/agentjit/internal/ingest"
	"github.com/spf13/cobra"
)

var ingestCmd = &cobra.Command{
	Use:    "ingest",
	Short:  "Receive hook JSON on stdin and forward to daemon",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := config.DefaultPaths()
		if err != nil {
			return err
		}

		cfg, err := config.Load(paths.Config)
		if err != nil {
			cfg = config.DefaultConfig()
		}

		return ingest.IngestFromReader(os.Stdin, paths, cfg)
	},
}

func init() {
	rootCmd.AddCommand(ingestCmd)
}
