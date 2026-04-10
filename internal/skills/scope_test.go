package skills

import (
	"testing"

	"github.com/agent-jit/agentjit/internal/config"
	"github.com/agent-jit/agentjit/internal/ingest"
)

func TestInferScopeMultipleProjects(t *testing.T) {
	cfg := config.DefaultConfig()

	events := []ingest.Event{
		{WorkingDirectory: "/Users/dev/project-a", ToolInput: map[string]interface{}{"command": "kubectl logs"}},
		{WorkingDirectory: "/Users/dev/project-b", ToolInput: map[string]interface{}{"command": "kubectl logs"}},
	}

	scope := InferScope(events, cfg.Scope)
	if scope != "global" {
		t.Errorf("scope = %q, want global (multi-project)", scope)
	}
}

func TestInferScopeSingleProject(t *testing.T) {
	cfg := config.DefaultConfig()

	events := []ingest.Event{
		{WorkingDirectory: "/Users/dev/project-a", ToolInput: map[string]interface{}{"command": "go test ./..."}},
		{WorkingDirectory: "/Users/dev/project-a", ToolInput: map[string]interface{}{"command": "go build"}},
	}

	scope := InferScope(events, cfg.Scope)
	if scope != "local" {
		t.Errorf("scope = %q, want local (single project)", scope)
	}
}

func TestInferScopeGlobalCLI(t *testing.T) {
	cfg := config.DefaultConfig()

	events := []ingest.Event{
		{WorkingDirectory: "/Users/dev/project-a", ToolInput: map[string]interface{}{"command": "kubectl get pods"}},
		{WorkingDirectory: "/Users/dev/project-a", ToolInput: map[string]interface{}{"command": "az aks get-credentials"}},
	}

	scope := InferScope(events, cfg.Scope)
	if scope != "global" {
		t.Errorf("scope = %q, want global (global CLI tools)", scope)
	}
}

func TestInferScopeProjectRoot(t *testing.T) {
	root := InferProjectRoot([]ingest.Event{
		{WorkingDirectory: "/Users/dev/myapp"},
		{WorkingDirectory: "/Users/dev/myapp/src"},
		{WorkingDirectory: "/Users/dev/myapp/tests"},
	})

	if root != "/Users/dev/myapp" {
		t.Errorf("project root = %q, want /Users/dev/myapp", root)
	}
}
