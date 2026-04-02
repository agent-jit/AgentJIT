package main

import (
	"fmt"
	"os"

	"github.com/anthropics/agentjit/internal/version"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:     "aj",
	Short:   "Background JIT compiler for autonomous coding agents",
	Long:    "AJ silently ingests agent execution telemetry, identifies recurring patterns, and compiles them into parameterized skills.",
	Version: version.Version,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
