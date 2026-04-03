package compile

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/anthropics/agentjit/internal/config"
	"github.com/anthropics/agentjit/internal/skills"
	"github.com/anthropics/agentjit/internal/stats"
)

// SessionSummary is a lightweight summary of a single session log file.
type SessionSummary struct {
	SessionID        string   `json:"session_id"`
	Date             string   `json:"date"`
	FilePath         string   `json:"file_path"`
	EventCount       int      `json:"event_count"`
	ToolNames        []string `json:"tool_names"`
	WorkingDirectory string   `json:"working_directory"`
}

// Manifest is the concise overview given to the compiler.
type Manifest struct {
	LogsDir       string           `json:"logs_dir"`
	SkillsDir     string           `json:"skills_dir"`
	TotalSessions int              `json:"total_sessions"`
	TotalEvents   int              `json:"total_events"`
	DateRange     [2]string        `json:"date_range"`
	Sessions      []SessionSummary `json:"sessions"`
}

// BuildManifest scans log directories and produces a lightweight manifest
// instead of loading all events into memory.
func BuildManifest(paths config.Paths) (Manifest, error) {
	marker, _ := ReadMarker(paths.CompileMarker)

	m := Manifest{
		LogsDir:   paths.Logs,
		SkillsDir: paths.Skills,
	}

	dateDirs, err := os.ReadDir(paths.Logs)
	if err != nil {
		if os.IsNotExist(err) {
			return m, nil
		}
		return m, fmt.Errorf("reading logs dir: %w", err)
	}

	sort.Slice(dateDirs, func(i, j int) bool {
		return dateDirs[i].Name() < dateDirs[j].Name()
	})

	for _, dateDir := range dateDirs {
		if !dateDir.IsDir() {
			continue
		}
		dateName := dateDir.Name()

		// Skip dirs before marker date
		if !marker.IsZero() {
			dirDate, err := time.Parse("2006-01-02", dateName)
			if err != nil {
				continue
			}
			if dirDate.Before(marker.Truncate(24 * time.Hour)) {
				continue
			}
		}

		dirPath := filepath.Join(paths.Logs, dateName)
		sessionFiles, err := os.ReadDir(dirPath)
		if err != nil {
			continue
		}

		for _, sf := range sessionFiles {
			if filepath.Ext(sf.Name()) != ".jsonl" {
				continue
			}

			filePath := filepath.Join(dirPath, sf.Name())
			summary := summarizeSession(filePath, dateName)
			if summary.EventCount == 0 {
				continue
			}

			m.Sessions = append(m.Sessions, summary)
			m.TotalEvents += summary.EventCount
		}
	}

	m.TotalSessions = len(m.Sessions)
	if m.TotalSessions > 0 {
		m.DateRange = [2]string{m.Sessions[0].Date, m.Sessions[m.TotalSessions-1].Date}
	}

	return m, nil
}

// summarizeSession reads a JSONL file and extracts a lightweight summary
// without keeping all events in memory.
func summarizeSession(filePath, date string) SessionSummary {
	sessionID := strings.TrimSuffix(filepath.Base(filePath), ".jsonl")
	summary := SessionSummary{
		SessionID: sessionID,
		Date:      date,
		FilePath:  filePath,
	}

	data, err := os.ReadFile(filePath)
	if err != nil {
		return summary
	}

	toolSet := make(map[string]bool)
	lines := strings.Split(strings.TrimSpace(string(data)), "\n")
	summary.EventCount = len(lines)

	for _, line := range lines {
		var partial struct {
			ToolName         string `json:"tool_name"`
			WorkingDirectory string `json:"working_directory"`
		}
		if json.Unmarshal([]byte(line), &partial) == nil {
			if partial.ToolName != "" {
				toolSet[partial.ToolName] = true
			}
			if partial.WorkingDirectory != "" && summary.WorkingDirectory == "" {
				summary.WorkingDirectory = partial.WorkingDirectory
			}
		}
	}

	for tool := range toolSet {
		summary.ToolNames = append(summary.ToolNames, tool)
	}
	sort.Strings(summary.ToolNames)

	return summary
}

// BuildPrompt takes a prompt template string and replaces config variables.
func BuildPrompt(promptTemplate string, cfg config.Config, globalSkillsDir string) string {
	prompt := promptTemplate

	replacements := map[string]string{
		"{{MIN_PATTERN_FREQUENCY}}":    strconv.Itoa(cfg.Compile.MinPatternFrequency),
		"{{MIN_TOKEN_SAVINGS}}":        strconv.Itoa(cfg.Compile.MinTokenSavings),
		"{{DEPRECATE_AFTER_SESSIONS}}": strconv.Itoa(cfg.Compile.DeprecateAfterSessions),
		"{{GLOBAL_SKILLS_DIR}}":        globalSkillsDir,
		"{{GLOBAL_CLI_TOOLS}}":         strings.Join(cfg.Scope.GlobalCLITools, ", "),
	}

	for key, val := range replacements {
		prompt = strings.ReplaceAll(prompt, key, val)
	}

	return prompt
}

// claudeOutput is the JSON envelope from `claude --print --output-format json`.
type claudeOutput struct {
	Result       string  `json:"result"`
	TotalCostUSD float64 `json:"total_cost_usd"`
	Usage        struct {
		InputTokens              int `json:"input_tokens"`
		OutputTokens             int `json:"output_tokens"`
		CacheCreationInputTokens int `json:"cache_creation_input_tokens"`
		CacheReadInputTokens     int `json:"cache_read_input_tokens"`
	} `json:"usage"`
	NumTurns   int   `json:"num_turns"`
	DurationMs int64 `json:"duration_ms"`
	SessionID  string `json:"session_id"`
}

// parseClaudeOutput parses the JSON output from `claude --print --output-format json`.
func parseClaudeOutput(data []byte) (claudeOutput, error) {
	var out claudeOutput
	if err := json.Unmarshal(data, &out); err != nil {
		return out, fmt.Errorf("parsing claude output: %w", err)
	}
	return out, nil
}

// RunCompile executes the full compilation sequence.
func RunCompile(paths config.Paths, cfg config.Config, promptTemplate string) error {
	start := time.Now()

	// 1. Build manifest
	fmt.Print("[AJ] Scanning logs... ")
	manifest, err := BuildManifest(paths)
	if err != nil {
		return fmt.Errorf("building manifest: %w", err)
	}

	if manifest.TotalSessions == 0 {
		fmt.Println("no new sessions to process")
		return nil
	}
	fmt.Printf("%d sessions, %d events (%s to %s)\n",
		manifest.TotalSessions, manifest.TotalEvents,
		manifest.DateRange[0], manifest.DateRange[1])

	// 2. Scan existing skills
	fmt.Print("[AJ] Scanning existing skills... ")
	existingSkills, _ := skills.ScanSkillsDir(paths.Skills)
	fmt.Printf("%d skills\n", len(existingSkills))

	// 3. Build prompt template with config values substituted
	prompt := BuildPrompt(promptTemplate, cfg, paths.Skills)

	// 4. Write manifest and compiled prompt to files
	manifestData, _ := json.MarshalIndent(manifest, "", "  ")
	manifestPath := filepath.Join(paths.Root, "compile-manifest.json")
	if err := os.WriteFile(manifestPath, manifestData, 0644); err != nil {
		return fmt.Errorf("writing manifest: %w", err)
	}

	compiledPromptPath := filepath.Join(paths.Root, "compile-prompt.md")
	if err := os.WriteFile(compiledPromptPath, []byte(prompt), 0644); err != nil {
		return fmt.Errorf("writing compiled prompt: %w", err)
	}

	// 5. Invoke Claude
	homeDir, _ := os.UserHomeDir()
	sessionID := uuid.New().String()
	fmt.Printf("[AJ] Starting compilation\n")
	fmt.Printf("[AJ] Session: %s\n", sessionID)
	fmt.Printf("[AJ] Attach from another terminal:\n")
	fmt.Printf("  cd ~ && claude --resume %s\n", sessionID)

	// Snapshot existing skills before compilation
	skillsBefore, _ := skills.ScanSkillsDir(paths.Skills)
	skillNamesBefore := make(map[string]bool, len(skillsBefore))
	for _, s := range skillsBefore {
		skillNamesBefore[s.Name] = true
	}

	userPrompt := fmt.Sprintf(
		"Read your compiler instructions from %s. "+
			"Then read the manifest at %s — it describes the available log files. "+
			"Use Glob, Grep, and Read to explore the JSONL log files as needed for pattern detection. "+
			"Do NOT try to read all logs at once — sample strategically. "+
			"Write generated skills to %s.",
		compiledPromptPath, manifestPath, paths.Skills)

	// Set up signal handling so Ctrl+C kills the Claude subprocess
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	cmd := exec.CommandContext(ctx, "claude",
		"--print",
		"--output-format", "json",
		"--session-id", sessionID,
		"--name", "aj-compile",
		"--allowedTools", "Read,Write,Bash,Glob,Grep",
		"--add-dir", paths.Root,
		"--add-dir", paths.Skills,
		"--add-dir", paths.Logs,
		"-p", userPrompt,
	)
	cmd.Dir = homeDir
	setProcGroup(cmd)
	var stdoutBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = os.Stderr

	// Handle signals in background — kill subprocess and write marker
	go func() {
		sig, ok := <-sigCh
		if !ok {
			return
		}
		fmt.Printf("\n[AJ] Received %s, stopping Claude subprocess...\n", sig)
		cancel()
		killProcGroup(cmd)
	}()

	runErr := cmd.Run()
	signal.Stop(sigCh)
	close(sigCh)

	if runErr != nil {
		// Still write the marker so next compile is incremental
		_ = WriteMarker(paths.CompileMarker, time.Now().UTC())
		return fmt.Errorf("running claude: %w", runErr)
	}

	// Parse token usage from JSON output
	output, parseErr := parseClaudeOutput(stdoutBuf.Bytes())
	if parseErr != nil {
		log.Printf("[AJ] Could not parse token metrics (upgrade Claude Code for token tracking): %v", parseErr)
	} else {
		// Print the result text
		if output.Result != "" {
			fmt.Println(output.Result)
		}
		fmt.Printf("[AJ] Compilation tokens: %d in / %d out ($%.2f)\n",
			output.Usage.InputTokens, output.Usage.OutputTokens, output.TotalCostUSD)
	}

	// Count skills created/updated
	skillsAfter, _ := skills.ScanSkillsDir(paths.Skills)
	var skillsCreated, skillsUpdated int
	for _, s := range skillsAfter {
		if skillNamesBefore[s.Name] {
			skillsUpdated++
		} else {
			skillsCreated++
		}
	}

	// Record compile stats
	if parseErr == nil {
		if err := stats.AppendCompileSession(paths.Stats, stats.CompileSessionData{
			SessionID:           sessionID,
			InputTokens:         output.Usage.InputTokens,
			OutputTokens:        output.Usage.OutputTokens,
			CacheCreationTokens: output.Usage.CacheCreationInputTokens,
			CacheReadTokens:     output.Usage.CacheReadInputTokens,
			TotalCostUSD:        output.TotalCostUSD,
			DurationMs:          output.DurationMs,
			NumTurns:            output.NumTurns,
			SkillsCreated:       skillsCreated,
			SkillsUpdated:       skillsUpdated,
			SessionsProcessed:   manifest.TotalSessions,
			EventsProcessed:     manifest.TotalEvents,
		}); err != nil {
			log.Printf("[AJ] Failed to record compile stats: %v", err)
		}
	}

	// Sync skill symlinks to Claude Code skills directory
	claudeSkillsDir, err := config.ClaudeSkillsGlobal()
	if err == nil {
		if err := skills.SyncLinks(paths.Skills, claudeSkillsDir); err != nil {
			log.Printf("[AJ] Failed to sync skill links: %v", err)
		}
	}

	// 6. Update marker
	if err := WriteMarker(paths.CompileMarker, time.Now().UTC()); err != nil {
		return fmt.Errorf("writing marker: %w", err)
	}

	elapsed := time.Since(start).Round(time.Second)
	fmt.Printf("[AJ] Compilation complete (%s)\n", elapsed)
	return nil
}
