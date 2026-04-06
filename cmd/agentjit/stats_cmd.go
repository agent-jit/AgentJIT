package main

import (
	"encoding/json"
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

var (
	recordSkill   string
	recordSuccess bool
	recordSession string
)

var statsRecordCmd = &cobra.Command{
	Use:    "record",
	Short:  "Record a skill execution (called by companion scripts)",
	Hidden: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		if recordSkill == "" {
			return fmt.Errorf("--skill is required")
		}

		paths, err := config.DefaultPaths()
		if err != nil {
			return err
		}

		// Read savings estimate from metadata.json
		var tokensSaved int
		metaPath := paths.Skills + "/" + recordSkill + "/metadata.json"
		if data, err := os.ReadFile(metaPath); err == nil {
			var meta struct {
				ROI struct {
					SavingsPerInvocation int `json:"savings_per_invocation"`
				} `json:"roi"`
			}
			if json.Unmarshal(data, &meta) == nil {
				tokensSaved = meta.ROI.SavingsPerInvocation
			}
		}

		return stats.AppendSkillExecution(paths.Stats, stats.SkillExecutionData{
			SkillName:            recordSkill,
			Success:              recordSuccess,
			EstimatedTokensSaved: tokensSaved,
			SessionID:            recordSession,
		})
	},
}

func init() {
	statsCmd.Flags().BoolVar(&statsJSON, "json", false, "Output stats as JSON")
	statsRecordCmd.Flags().StringVar(&recordSkill, "skill", "", "Skill name to record")
	statsRecordCmd.Flags().BoolVar(&recordSuccess, "success", true, "Whether execution succeeded")
	statsRecordCmd.Flags().StringVar(&recordSession, "session", "", "Claude session ID")
	statsCmd.AddCommand(statsResetCmd, statsRecordCmd)
	rootCmd.AddCommand(statsCmd)
}
