package stats

import (
	"encoding/json"
	"log"
	"os"
	"path/filepath"

	"github.com/agent-jit/agentjit/internal/config"
)

// CheckSkillExecution examines tool event fields and, if they represent
// an AJ-generated skill execution, records it to the stats file.
func CheckSkillExecution(toolName, eventType, sessionID string, toolInput map[string]interface{}, paths config.Paths) {
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
		EstimatedTokensSaved: tokensSaved,
		SessionID:            sessionID,
	}); err != nil {
		log.Printf("[AJ] stats: failed to record skill execution: %v", err)
	}
}
