package compile

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"

	"github.com/anthropics/agentjit/internal/config"
	"github.com/anthropics/agentjit/internal/skills"
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

// BuildPrompt reads the compiler prompt template and replaces config variables.
func BuildPrompt(promptPath string, cfg config.Config, globalSkillsDir string) (string, error) {
	data, err := os.ReadFile(promptPath)
	if err != nil {
		return "", fmt.Errorf("reading prompt: %w", err)
	}

	prompt := string(data)

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

	return prompt, nil
}

// RunCompile executes the full compilation sequence.
func RunCompile(paths config.Paths, cfg config.Config, promptPath string) error {
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
	prompt, err := BuildPrompt(promptPath, cfg, paths.Skills)
	if err != nil {
		return fmt.Errorf("building prompt: %w", err)
	}

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

	userPrompt := fmt.Sprintf(
		"Read your compiler instructions from %s. "+
			"Then read the manifest at %s — it describes the available log files. "+
			"Use Glob, Grep, and Read to explore the JSONL log files as needed for pattern detection. "+
			"Do NOT try to read all logs at once — sample strategically. "+
			"Write generated skills to %s.",
		compiledPromptPath, manifestPath, paths.Skills)

	cmd := exec.Command("claude",
		"--print",
		"--session-id", sessionID,
		"--name", "aj-compile",
		"--allowedTools", "Read,Write,Bash,Glob,Grep",
		"--add-dir", paths.Root,
		"--add-dir", paths.Skills,
		"--add-dir", paths.Logs,
		"-p", userPrompt,
	)
	cmd.Dir = homeDir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("running claude: %w", err)
	}

	// 6. Update marker
	if err := WriteMarker(paths.CompileMarker, time.Now().UTC()); err != nil {
		return fmt.Errorf("writing marker: %w", err)
	}

	elapsed := time.Since(start).Round(time.Second)
	fmt.Printf("[AJ] Compilation complete (%s)\n", elapsed)
	return nil
}
