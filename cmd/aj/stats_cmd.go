package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"time"

	"github.com/agent-jit/agentjit/internal/compile"
	"github.com/agent-jit/agentjit/internal/config"
	"github.com/agent-jit/agentjit/internal/stats"
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
		cfg, err := config.Load(paths.Config)
		if err != nil {
			cfg = config.DefaultConfig()
		}
		nextInfo := getNextCompileInfo(paths, cfg)
		return stats.PrintStats(paths.Stats, nextInfo, statsJSON)
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
	recordSkill           string
	recordSuccess         bool
	recordSession         string
	recordFailureCategory string
	recordFailureReason   string
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
			FailureCategory:      recordFailureCategory,
			FailureReason:        recordFailureReason,
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
	statsRecordCmd.Flags().StringVar(&recordFailureCategory, "failure-category", "", "Failure category (script_error or target_failure)")
	statsRecordCmd.Flags().StringVar(&recordFailureReason, "failure-reason", "", "Failure reason description")
	statsCmd.AddCommand(statsResetCmd, statsRecordCmd)
	rootCmd.AddCommand(statsCmd)
}

func getNextCompileInfo(paths config.Paths, cfg config.Config) *stats.NextCompileInfo {
	info := &stats.NextCompileInfo{
		TriggerMode: cfg.Compile.TriggerMode,
	}

	if cfg.Compile.TriggerMode == "manual" {
		return info
	}

	eventCount, markerTime, err := compile.CountEventsSinceMarker(paths)
	if err != nil {
		return info
	}

	if !markerTime.IsZero() {
		info.LastCompileTime = &markerTime
	}

	switch cfg.Compile.TriggerMode {
	case "interval":
		info.IntervalMinutes = cfg.Compile.TriggerIntervalMinutes
		if !markerTime.IsZero() {
			elapsed := time.Since(markerTime)
			remaining := time.Duration(cfg.Compile.TriggerIntervalMinutes)*time.Minute - elapsed
			mins := int(math.Ceil(remaining.Minutes()))
			if mins < 0 {
				mins = 0
			}
			info.MinutesRemaining = &mins
		}
	case "event_count":
		info.EventThreshold = cfg.Compile.TriggerEventThreshold
		info.EventsSinceCompile = eventCount
		remaining := cfg.Compile.TriggerEventThreshold - eventCount
		if remaining < 0 {
			remaining = 0
		}
		info.EventsRemaining = &remaining
	}

	return info
}
