package compile

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/agent-jit/agentjit/internal/config"
	"github.com/agent-jit/agentjit/internal/trace"
)

// DeterministicBackend compiles parameterized patterns into deterministic
// shell scripts (bash + PowerShell) rather than relying on an LLM.
type DeterministicBackend struct {
	skillsDir string
	platform  string
}

// NewDeterministicBackend creates a new deterministic codegen backend.
func NewDeterministicBackend(paths config.Paths, platform string) *DeterministicBackend {
	return &DeterministicBackend{
		skillsDir: paths.Skills,
		platform:  platform,
	}
}

// Name returns the backend identifier.
func (b *DeterministicBackend) Name() string { return "aj-deterministic" }

// Compile iterates patterns and produces a skill for each.
func (b *DeterministicBackend) Compile(ctx context.Context, patterns []trace.Pattern) ([]SkillResult, error) {
	var results []SkillResult
	for _, p := range patterns {
		r, err := b.compilePattern(p)
		if err != nil {
			return nil, fmt.Errorf("compilePattern: %w", err)
		}
		results = append(results, r)
	}
	return results, nil
}

// compilePattern creates a skill directory with scripts and metadata for
// a single pattern.
func (b *DeterministicBackend) compilePattern(p trace.Pattern) (SkillResult, error) {
	name := inferSkillName(p)
	skillDir := filepath.Join(b.skillsDir, name)
	scriptsDir := filepath.Join(skillDir, "scripts")

	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		return SkillResult{}, fmt.Errorf("mkdir scripts: %w", err)
	}

	// Generate and write bash script.
	bashScript := generateBashScript(p.Steps)
	bashPath := filepath.Join(scriptsDir, name+".sh")
	if err := os.WriteFile(bashPath, []byte(bashScript), 0755); err != nil {
		return SkillResult{}, fmt.Errorf("write bash script: %w", err)
	}

	// Generate and write PowerShell script.
	psScript := generatePowerShellScript(p.Steps)
	psPath := filepath.Join(scriptsDir, name+".ps1")
	if err := os.WriteFile(psPath, []byte(psScript), 0644); err != nil {
		return SkillResult{}, fmt.Errorf("write powershell script: %w", err)
	}

	// Generate and write SKILL.md.
	skillMD := generateSkillMD(name, p)
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(skillMD), 0644); err != nil {
		return SkillResult{}, fmt.Errorf("write SKILL.md: %w", err)
	}

	// Generate and write metadata.json.
	roi := calculateROI(p)
	meta := map[string]interface{}{
		"generated_by":  "aj-deterministic",
		"generated_at":  time.Now().UTC().Format(time.RFC3339),
		"frequency":     p.Frequency,
		"session_count": len(p.SessionIDs),
		"step_count":    len(p.Steps),
		"roi":           roi,
	}
	metaBytes, err := json.MarshalIndent(meta, "", "  ")
	if err != nil {
		return SkillResult{}, fmt.Errorf("marshal metadata: %w", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "metadata.json"), metaBytes, 0644); err != nil {
		return SkillResult{}, fmt.Errorf("write metadata.json: %w", err)
	}

	return SkillResult{
		Name:      name,
		Path:      skillDir,
		CreatedBy: "aj-deterministic",
	}, nil
}

// generateBashScript produces a bash script from the pattern steps.
// Non-Bash steps are skipped.
func generateBashScript(steps []trace.PatternStep) string {
	params := collectParams(steps)
	var b strings.Builder

	b.WriteString("#!/usr/bin/env bash\n")
	b.WriteString("set -euo pipefail\n\n")

	// Usage validation for parameters.
	if len(params) > 0 {
		b.WriteString(fmt.Sprintf("if [ $# -lt %d ]; then\n", len(params)))
		paramNames := make([]string, len(params))
		for i, p := range params {
			paramNames[i] = p.Name
		}
		b.WriteString(fmt.Sprintf("  echo \"Usage: $0 %s\" >&2\n", strings.Join(paramNames, " ")))
		b.WriteString("  exit 1\n")
		b.WriteString("fi\n\n")

		// Map positional args to named variables.
		for i, p := range params {
			b.WriteString(fmt.Sprintf("%s=\"${%d}\"\n", p.Name, i+1))
		}
		b.WriteString("\n")
	}

	// Emit steps, skipping non-Bash.
	stepNum := 0
	for _, s := range steps {
		if s.ToolName != "Bash" {
			continue
		}
		stepNum++
		b.WriteString(fmt.Sprintf("# Step %d\n", stepNum))
		b.WriteString(s.Template + "\n\n")
	}

	return b.String()
}

// generatePowerShellScript produces a PowerShell script from the pattern steps.
// Non-Bash steps are skipped.
func generatePowerShellScript(steps []trace.PatternStep) string {
	params := collectParams(steps)
	var b strings.Builder

	b.WriteString("$ErrorActionPreference = 'Stop'\n")
	b.WriteString("$PSNativeCommandUseErrorActionPreference = $true\n\n")

	// Param block for parameters.
	if len(params) > 0 {
		b.WriteString("param(\n")
		for i, p := range params {
			mandatory := "    [Parameter(Mandatory=$true)]\n"
			b.WriteString(mandatory)
			suffix := ","
			if i == len(params)-1 {
				suffix = ""
			}
			b.WriteString(fmt.Sprintf("    [string]$%s%s\n", p.Name, suffix))
		}
		b.WriteString(")\n\n")
	}

	// Emit steps, skipping non-Bash.
	stepNum := 0
	for _, s := range steps {
		if s.ToolName != "Bash" {
			continue
		}
		stepNum++
		b.WriteString(fmt.Sprintf("# Step %d\n", stepNum))
		// Replace $VAR with $VAR (PowerShell uses same syntax for variables).
		b.WriteString(s.Template + "\n\n")
	}

	return b.String()
}

// collectParams returns a deduplicated list of parameters across all steps,
// preserving first-seen order.
func collectParams(steps []trace.PatternStep) []trace.Parameter {
	seen := make(map[string]bool)
	var params []trace.Parameter
	for _, s := range steps {
		for _, p := range s.Parameters {
			if !seen[p.Name] {
				seen[p.Name] = true
				params = append(params, p)
			}
		}
	}
	return params
}

// ROI holds the return-on-investment estimate for a deterministic skill.
type ROI struct {
	StochasticTokensPerRun int `json:"stochastic_tokens_per_run"`
	DeterministicOverhead  int `json:"deterministic_overhead"`
	TokensSavedPerRun      int `json:"tokens_saved_per_run"`
}

// calculateROI estimates token savings for a pattern compiled deterministically.
// Assumes 200 tokens per step for stochastic execution and 200 fixed overhead
// for the deterministic wrapper.
func calculateROI(p trace.Pattern) ROI {
	stochastic := len(p.Steps) * 200
	overhead := 200
	saved := stochastic - overhead
	if saved < 0 {
		saved = 0
	}
	return ROI{
		StochasticTokensPerRun: stochastic,
		DeterministicOverhead:  overhead,
		TokensSavedPerRun:      saved,
	}
}

// inferSkillName generates a skill directory name from the first Bash step's
// template. It takes the first 2-3 non-flag, non-parameter tokens.
func inferSkillName(p trace.Pattern) string {
	for _, s := range p.Steps {
		if s.ToolName != "Bash" {
			continue
		}
		tokens := strings.Fields(s.Template)
		var parts []string
		for _, tok := range tokens {
			if strings.HasPrefix(tok, "-") {
				continue
			}
			if strings.HasPrefix(tok, "$") {
				continue
			}
			parts = append(parts, tok)
			if len(parts) >= 3 {
				break
			}
		}
		if len(parts) > 0 {
			name := strings.Join(parts, "-")
			// Sanitize: replace non-alphanumeric (except dash) with dash.
			name = sanitizeName(name)
			return name
		}
	}
	return "unnamed-skill"
}

// sanitizeName replaces characters that are not alphanumeric or dashes with dashes,
// and collapses consecutive dashes.
func sanitizeName(s string) string {
	var b strings.Builder
	prevDash := false
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
			b.WriteRune(c)
			prevDash = false
		} else {
			if !prevDash {
				b.WriteRune('-')
				prevDash = true
			}
		}
	}
	return strings.Trim(b.String(), "-")
}

// generateSkillMD produces SKILL.md content with YAML frontmatter describing
// the skill's trigger, parameters, and execution.
func generateSkillMD(name string, p trace.Pattern) string {
	params := collectParams(p.Steps)
	var b strings.Builder

	// YAML frontmatter.
	b.WriteString("---\n")
	b.WriteString(fmt.Sprintf("name: %s\n", name))
	b.WriteString(fmt.Sprintf("trigger: %s\n", inferTriggerClause(p)))
	b.WriteString("generated_by: aj-deterministic\n")
	b.WriteString("---\n\n")

	// Description.
	b.WriteString(fmt.Sprintf("# %s\n\n", name))
	b.WriteString(describePattern(p) + "\n\n")

	// Parameters section.
	if len(params) > 0 {
		b.WriteString("## Parameters\n\n")
		for _, param := range params {
			b.WriteString(fmt.Sprintf("- **%s**: %s\n", param.Name, describeParam(param)))
		}
		b.WriteString("\n")
	}

	// Execution section.
	b.WriteString("## Execution\n\n")
	b.WriteString("This skill runs a deterministic sequence of shell commands.\n")
	b.WriteString(fmt.Sprintf("Total steps: %d\n", countBashSteps(p.Steps)))

	return b.String()
}

// inferTriggerClause generates a trigger description from the pattern.
func inferTriggerClause(p trace.Pattern) string {
	for _, s := range p.Steps {
		if s.ToolName != "Bash" {
			continue
		}
		tokens := strings.Fields(s.Template)
		if len(tokens) > 0 {
			cmd := tokens[0]
			if len(tokens) > 1 && !strings.HasPrefix(tokens[1], "-") && !strings.HasPrefix(tokens[1], "$") {
				return fmt.Sprintf("When user runs %s %s", cmd, tokens[1])
			}
			return fmt.Sprintf("When user runs %s", cmd)
		}
	}
	return "When user requests this workflow"
}

// describePattern generates a human-readable description of the pattern.
func describePattern(p trace.Pattern) string {
	bashCount := countBashSteps(p.Steps)
	return fmt.Sprintf("Automated workflow with %d command(s), observed %d times across %d sessions.",
		bashCount, p.Frequency, len(p.SessionIDs))
}

// describeParam generates a human-readable description of a parameter.
func describeParam(p trace.Parameter) string {
	if len(p.Values) == 0 {
		return "User-provided value"
	}
	if len(p.Values) <= 3 {
		return fmt.Sprintf("Observed values: %s", strings.Join(p.Values, ", "))
	}
	return fmt.Sprintf("Observed values: %s, ... (%d total)",
		strings.Join(p.Values[:3], ", "), len(p.Values))
}

// countBashSteps returns the number of Bash steps in the step list.
func countBashSteps(steps []trace.PatternStep) int {
	count := 0
	for _, s := range steps {
		if s.ToolName == "Bash" {
			count++
		}
	}
	return count
}
