package skills

import (
	"path/filepath"
	"strings"

	"github.com/agent-jit/agentjit/internal/config"
	"github.com/agent-jit/agentjit/internal/ingest"
)

// InferScope determines whether a pattern should be registered as global or local.
func InferScope(events []ingest.Event, scopeCfg config.ScopeConfig) string {
	// Count distinct project roots
	roots := make(map[string]bool)
	for _, e := range events {
		root := projectRoot(e.WorkingDirectory)
		roots[root] = true
	}

	if len(roots) >= scopeCfg.CrossProjectThreshold {
		return "global"
	}

	// Fallback: check if commands primarily use global CLIs
	globalCount := 0
	totalCount := 0
	for _, e := range events {
		cmd, ok := e.ToolInput["command"].(string)
		if !ok {
			continue
		}
		totalCount++
		for _, tool := range scopeCfg.GlobalCLITools {
			if strings.HasPrefix(cmd, tool+" ") || cmd == tool {
				globalCount++
				break
			}
		}
	}

	if totalCount > 0 && float64(globalCount)/float64(totalCount) > 0.5 {
		return "global"
	}

	return "local"
}

// InferProjectRoot finds the common root directory from a set of events.
func InferProjectRoot(events []ingest.Event) string {
	if len(events) == 0 {
		return ""
	}

	common := events[0].WorkingDirectory
	for _, e := range events[1:] {
		common = commonPrefix(common, e.WorkingDirectory)
	}

	return filepath.Clean(common)
}

// projectRoot extracts a coarse project root (first 4 path components).
func projectRoot(dir string) string {
	parts := strings.Split(filepath.Clean(dir), string(filepath.Separator))
	if len(parts) > 4 {
		parts = parts[:4]
	}
	return strings.Join(parts, string(filepath.Separator))
}

func commonPrefix(a, b string) string {
	aParts := strings.Split(a, string(filepath.Separator))
	bParts := strings.Split(b, string(filepath.Separator))

	var common []string
	for i := 0; i < len(aParts) && i < len(bParts); i++ {
		if aParts[i] != bParts[i] {
			break
		}
		common = append(common, aParts[i])
	}

	return strings.Join(common, string(filepath.Separator))
}
