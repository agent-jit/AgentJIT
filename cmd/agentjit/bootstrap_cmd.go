package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/anthropics/agentjit/internal/bootstrap"
	"github.com/anthropics/agentjit/internal/config"
	"github.com/spf13/cobra"
)

var bootstrapSince string
var bootstrapProject string
var bootstrapDryRun bool

var bootstrapCmd = &cobra.Command{
	Use:   "bootstrap",
	Short: "Import historical Claude Code transcripts into logs",
	RunE: func(cmd *cobra.Command, args []string) error {
		paths, err := config.DefaultPaths()
		if err != nil {
			return err
		}
		if err := paths.EnsureDirs(); err != nil {
			return fmt.Errorf("creating directories: %w", err)
		}

		cfg, err := config.Load(paths.Config)
		if err != nil {
			cfg = config.DefaultConfig()
		}

		claudeProjectsDir, err := config.ClaudeProjectsDir()
		if err != nil {
			return fmt.Errorf("finding Claude projects dir: %w", err)
		}

		opts := bootstrap.BootstrapOptions{
			Since:   bootstrapSince,
			Project: bootstrapProject,
			DryRun:  bootstrapDryRun,
		}

		if bootstrapDryRun {
			fmt.Println("[AJ] Dry run — no files will be written")
		}

		result, err := bootstrap.RunBootstrap(paths, cfg, claudeProjectsDir, opts)
		if err != nil {
			return err
		}

		if result.SessionsProcessed == 0 {
			fmt.Println("[AJ] No new sessions to import")
			return nil
		}

		fmt.Printf("[AJ] Bootstrapped %d sessions (%d events)\n",
			result.SessionsProcessed, result.EventsImported)

		if !bootstrapDryRun {
			fmt.Print("[AJ] Run compile now? [Y/n] ")
			reader := bufio.NewReader(os.Stdin)
			answer, _ := reader.ReadString('\n')
			answer = strings.TrimSpace(strings.ToLower(answer))

			if answer == "" || answer == "y" || answer == "yes" {
				return compileCmd.RunE(compileCmd, nil)
			}
		}

		return nil
	},
}

func init() {
	bootstrapCmd.Flags().StringVar(&bootstrapSince, "since", "", "Only transcripts after this date (YYYY-MM-DD)")
	bootstrapCmd.Flags().StringVar(&bootstrapProject, "project", "", "Only transcripts for this project path")
	bootstrapCmd.Flags().BoolVar(&bootstrapDryRun, "dry-run", false, "Show what would be processed without writing")
	rootCmd.AddCommand(bootstrapCmd)
}
