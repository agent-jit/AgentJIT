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

// AgentJITHooks returns the hook configuration for all AgentJIT hook events.
func AgentJITHooks() map[string][]MatcherGroup {
	return map[string][]MatcherGroup{
		"PostToolUse": {
			{
				Hooks: []HookHandler{
					{Type: "command", Command: "agentjit ingest", Async: true},
				},
			},
		},
		"PostToolUseFailure": {
			{
				Hooks: []HookHandler{
					{Type: "command", Command: "agentjit ingest", Async: true},
				},
			},
		},
		"SessionStart": {
			{
				Hooks: []HookHandler{
					{Type: "command", Command: "agentjit daemon start --if-not-running && agentjit ingest"},
				},
			},
		},
		"SessionEnd": {
			{
				Hooks: []HookHandler{
					{Type: "command", Command: "agentjit ingest", Async: true},
				},
			},
		},
	}
}

// isAgentJITHook checks if a hook handler belongs to AgentJIT.
func isAgentJITHook(command string) bool {
	return len(command) >= 14 && (command[:14] == "agentjit inges" || command[:14] == "agentjit daemo")
}
