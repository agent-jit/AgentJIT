package hooks

// HookHandler represents a single hook handler in Claude Code settings.
type HookHandler struct {
	Type    string `json:"type"`
	Command string `json:"command"`
	Async   bool   `json:"async,omitempty"`
}

// MatcherGroup represents a matcher group containing handlers.
type MatcherGroup struct {
	Matcher string        `json:"matcher,omitempty"`
	Hooks   []HookHandler `json:"hooks"`
}

// AJHooks returns the hook configuration for all AJ hook events.
func AJHooks() map[string][]MatcherGroup {
	return map[string][]MatcherGroup{
		"PostToolUse": {
			{
				Hooks: []HookHandler{
					{Type: "command", Command: "aj ingest", Async: true},
				},
			},
		},
		"PostToolUseFailure": {
			{
				Hooks: []HookHandler{
					{Type: "command", Command: "aj ingest", Async: true},
				},
			},
		},
		"SessionStart": {
			{
				Hooks: []HookHandler{
					{Type: "command", Command: "aj daemon start --if-not-running && aj ingest"},
				},
			},
		},
		"SessionEnd": {
			{
				Hooks: []HookHandler{
					{Type: "command", Command: "aj ingest", Async: true},
				},
			},
		},
	}
}

// isAJHook checks if a hook handler belongs to AJ.
func isAJHook(command string) bool {
	return len(command) >= 9 && (command[:9] == "aj ingest" || command[:9] == "aj daemon")
}
