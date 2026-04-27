package stats

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/agent-jit/agentjit/internal/config"
)

const maxFailureReasonLen = 256

// CheckSkillExecution examines tool event fields and, if they represent
// an AJ-generated skill execution, records it to the stats file.
func CheckSkillExecution(toolName, eventType, sessionID string, toolInput map[string]interface{}, toolError string, exitCode *int, paths config.Paths) {
	if toolName != "Skill" {
		return
	}

	skillName, ok := toolInput["skill"].(string)
	if !ok || skillName == "" {
		return
	}

	// Only track skills that exist in the AJ skills directory
	skillDir := filepath.Join(paths.Skills, skillName)
	if _, err := os.Stat(skillDir); os.IsNotExist(err) {
		return
	}

	success := eventType == "post_tool_use"

	var failureCategory, failureReason string
	if !success {
		failureCategory, failureReason = classifyFailure(toolError, exitCode)
	}

	// Read savings estimate from metadata.json
	var tokensSaved int
	metaPath := filepath.Join(skillDir, "metadata.json")
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

	if err := AppendSkillExecution(paths.Stats, SkillExecutionData{
		SkillName:            skillName,
		Success:              success,
		FailureCategory:      failureCategory,
		FailureReason:        failureReason,
		EstimatedTokensSaved: tokensSaved,
		SessionID:            sessionID,
	}); err != nil {
		log.Printf("[AJ] stats: failed to record skill execution: %v", err)
	}
}

// classifyFailure determines whether a failure is a script error (the skill
// itself broke) or a target failure (the downstream command returned non-zero).
func classifyFailure(toolError string, exitCode *int) (category, reason string) {
	if exitCode != nil && *exitCode != 0 && toolError == "" {
		return "target_failure", fmt.Sprintf("exit code %d", *exitCode)
	}

	if toolError != "" {
		reason = toolError
		if len(reason) > maxFailureReasonLen {
			reason = reason[:maxFailureReasonLen]
		}
		if exitCode != nil && *exitCode != 0 {
			return "target_failure", reason
		}
		return "script_error", reason
	}

	return "script_error", "unknown"
}
