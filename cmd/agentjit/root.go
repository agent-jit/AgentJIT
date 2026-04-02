package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "aj",
	Short: "Background JIT compiler for autonomous coding agents",
	Long:  "AJ silently ingests agent execution telemetry, identifies recurring patterns, and compiles them into parameterized skills.",
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
