package stats

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// RecordType identifies the kind of stats record.
type RecordType string

const (
	RecordCompileSession RecordType = "compile_session"
	RecordSkillExecution RecordType = "skill_execution"
)

// Record is the envelope written to stats.jsonl.
type Record struct {
	Type      RecordType      `json:"type"`
	Timestamp time.Time       `json:"timestamp"`
	Data      json.RawMessage `json:"data"`
}

// CompileSessionData holds metrics for a single compile invocation.
type CompileSessionData struct {
	SessionID               string  `json:"session_id"`
	InputTokens             int     `json:"input_tokens"`
	OutputTokens            int     `json:"output_tokens"`
	CacheCreationTokens     int     `json:"cache_creation_input_tokens"`
	CacheReadTokens         int     `json:"cache_read_input_tokens"`
	TotalCostUSD            float64 `json:"total_cost_usd"`
	DurationMs              int64   `json:"duration_ms"`
	NumTurns                int     `json:"num_turns"`
	SkillsCreated           int     `json:"skills_created"`
	SkillsUpdated           int     `json:"skills_updated"`
	SessionsProcessed       int     `json:"sessions_processed"`
	EventsProcessed         int     `json:"events_processed"`
	DeterministicPatterns   int     `json:"deterministic_patterns"`
	LLMPatterns             int     `json:"llm_patterns"`
}

// SkillExecutionData holds metrics for a single skill execution.
type SkillExecutionData struct {
	SkillName            string `json:"skill_name"`
	Success              bool   `json:"success"`
	EstimatedTokensSaved int    `json:"estimated_tokens_saved"`
	SessionID            string `json:"session_id,omitempty"`
}

// AppendRecord marshals and appends a record to the JSONL file.
func AppendRecord(filePath string, record Record) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("creating dir: %w", err)
	}

	f, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("opening stats file: %w", err)
	}
	defer f.Close()

	data, err := json.Marshal(record)
	if err != nil {
		return fmt.Errorf("marshaling record: %w", err)
	}

	data = append(data, '\n')
	_, err = f.Write(data)
	return err
}

// AppendCompileSession records a compile session to the stats file.
func AppendCompileSession(filePath string, data CompileSessionData) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return AppendRecord(filePath, Record{
		Type:      RecordCompileSession,
		Timestamp: time.Now().UTC(),
		Data:      raw,
	})
}

// AppendSkillExecution records a skill execution to the stats file.
func AppendSkillExecution(filePath string, data SkillExecutionData) error {
	raw, err := json.Marshal(data)
	if err != nil {
		return err
	}
	return AppendRecord(filePath, Record{
		Type:      RecordSkillExecution,
		Timestamp: time.Now().UTC(),
		Data:      raw,
	})
}

// ReadAllRecords reads all records from the stats JSONL file.
// Malformed lines are silently skipped.
func ReadAllRecords(filePath string) ([]Record, error) {
	f, err := os.Open(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer f.Close()

	var records []Record
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1024*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec Record
		if json.Unmarshal(line, &rec) == nil {
			records = append(records, rec)
		}
	}
	return records, scanner.Err()
}

// Aggregated holds aggregated stats for display.
type Aggregated struct {
	CompileSessions    int     `json:"compile_sessions"`
	CompileInputTokens int     `json:"compile_input_tokens"`
	CompileOutputTokens int    `json:"compile_output_tokens"`
	CompileTotalCost   float64 `json:"compile_total_cost_usd"`
	SkillExecutions    int     `json:"skill_executions"`
	SkillSuccesses     int     `json:"skill_successes"`
	SkillFailures      int     `json:"skill_failures"`
	EstTokensSaved     int     `json:"estimated_tokens_saved"`
}

// Aggregate computes aggregated stats from records.
func Aggregate(records []Record) Aggregated {
	var agg Aggregated
	for _, rec := range records {
		switch rec.Type {
		case RecordCompileSession:
			var d CompileSessionData
			if json.Unmarshal(rec.Data, &d) == nil {
				agg.CompileSessions++
				agg.CompileInputTokens += d.InputTokens
				agg.CompileOutputTokens += d.OutputTokens
				agg.CompileTotalCost += d.TotalCostUSD
			}
		case RecordSkillExecution:
			var d SkillExecutionData
			if json.Unmarshal(rec.Data, &d) == nil {
				agg.SkillExecutions++
				if d.Success {
					agg.SkillSuccesses++
				} else {
					agg.SkillFailures++
				}
				agg.EstTokensSaved += d.EstimatedTokensSaved
			}
		}
	}
	return agg
}

// NextCompileInfo describes progress toward the next auto-compile.
type NextCompileInfo struct {
	TriggerMode        string     `json:"trigger_mode"`
	LastCompileTime    *time.Time `json:"last_compile_time,omitempty"`
	IntervalMinutes    int        `json:"interval_minutes,omitempty"`
	MinutesRemaining   *int       `json:"minutes_remaining,omitempty"`
	EventThreshold     int        `json:"event_threshold,omitempty"`
	EventsSinceCompile int        `json:"events_since_compile,omitempty"`
	EventsRemaining    *int       `json:"events_remaining,omitempty"`
}

// PrintStats reads the stats file and prints a formatted dashboard.
func PrintStats(statsPath string, nextInfo *NextCompileInfo, asJSON bool) error {
	records, err := ReadAllRecords(statsPath)
	if err != nil {
		return err
	}

	if len(records) == 0 && nextInfo == nil {
		fmt.Println("[AJ] No stats recorded yet. Stats are collected during 'aj compile' and skill executions.")
		return nil
	}

	agg := Aggregate(records)

	if asJSON {
		combined := struct {
			Aggregated
			NextCompile *NextCompileInfo `json:"next_compile,omitempty"`
		}{Aggregated: agg, NextCompile: nextInfo}
		data, err := json.MarshalIndent(combined, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(data))
		return nil
	}

	totalSpent := agg.CompileInputTokens + agg.CompileOutputTokens
	netSavings := agg.EstTokensSaved - totalSpent
	var roi float64
	if totalSpent > 0 {
		roi = float64(agg.EstTokensSaved) / float64(totalSpent)
	}

	var avgCost float64
	if agg.CompileSessions > 0 {
		avgCost = agg.CompileTotalCost / float64(agg.CompileSessions)
	}

	var successRate float64
	if agg.SkillExecutions > 0 {
		successRate = float64(agg.SkillSuccesses) / float64(agg.SkillExecutions) * 100
	}

	fmt.Println("=== AJ Token Metrics ===")
	fmt.Println()
	fmt.Println("Compilation")
	fmt.Printf("  Sessions:          %d\n", agg.CompileSessions)
	fmt.Printf("  Input tokens:      %d\n", agg.CompileInputTokens)
	fmt.Printf("  Output tokens:     %d\n", agg.CompileOutputTokens)
	fmt.Printf("  Total cost:        $%.2f\n", agg.CompileTotalCost)
	fmt.Printf("  Avg cost/compile:  $%.2f\n", avgCost)
	fmt.Println()
	fmt.Println("Skill Executions")
	fmt.Printf("  Total:             %d\n", agg.SkillExecutions)
	fmt.Printf("  Successful:        %d (%.1f%%)\n", agg.SkillSuccesses, successRate)
	fmt.Printf("  Failed:            %d\n", agg.SkillFailures)
	fmt.Println()
	fmt.Println("Token Savings")
	fmt.Printf("  Est. tokens saved: %d\n", agg.EstTokensSaved)
	fmt.Printf("  Tokens spent:      %d\n", totalSpent)
	fmt.Printf("  Net savings:       %d\n", netSavings)
	fmt.Printf("  ROI:               %.2fx\n", roi)

	if nextInfo != nil {
		fmt.Println()
		fmt.Println("Next Compile")
		switch nextInfo.TriggerMode {
		case "manual":
			fmt.Println("  Trigger mode:      manual (run 'aj compile' to trigger)")
		case "interval":
			fmt.Printf("  Trigger mode:      interval\n")
			if nextInfo.LastCompileTime != nil {
				fmt.Printf("  Last compile:      %s\n", nextInfo.LastCompileTime.Format("2006-01-02 15:04:05 UTC"))
			} else {
				fmt.Printf("  Last compile:      never\n")
			}
			fmt.Printf("  Interval:          %dm\n", nextInfo.IntervalMinutes)
			if nextInfo.MinutesRemaining != nil {
				if *nextInfo.MinutesRemaining == 0 {
					fmt.Printf("  Status:            ready (waiting for next check)\n")
				} else {
					fmt.Printf("  Time remaining:    %dm\n", *nextInfo.MinutesRemaining)
				}
			}
		case "event_count":
			fmt.Printf("  Trigger mode:      event_count\n")
			fmt.Printf("  Events threshold:  %d\n", nextInfo.EventThreshold)
			fmt.Printf("  Events received:   %d\n", nextInfo.EventsSinceCompile)
			if nextInfo.EventsRemaining != nil {
				if *nextInfo.EventsRemaining == 0 {
					fmt.Printf("  Status:            ready (waiting for next check)\n")
				} else {
					fmt.Printf("  Events remaining:  %d\n", *nextInfo.EventsRemaining)
				}
			}
		}
	}

	return nil
}
