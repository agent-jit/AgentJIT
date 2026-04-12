package trace

import (
	"testing"
)

func TestTokenizeBashCommand_LiteralOnly(t *testing.T) {
	tokens := TokenizeBashCommand("ls -la")
	if len(tokens) != 2 {
		t.Fatalf("got %d tokens, want 2", len(tokens))
	}
	if tokens[0].Value != "ls" || !tokens[0].Literal {
		t.Errorf("token[0] = %+v, want literal 'ls'", tokens[0])
	}
	if tokens[1].Value != "-la" || !tokens[1].Literal {
		t.Errorf("token[1] = %+v, want literal '-la'", tokens[1])
	}
}

func TestTokenizeBashCommand_PathBecomesVar(t *testing.T) {
	tokens := TokenizeBashCommand("cat /home/user/project/main.go")
	if len(tokens) != 2 {
		t.Fatalf("got %d tokens, want 2", len(tokens))
	}
	if tokens[0].Value != "cat" || !tokens[0].Literal {
		t.Errorf("token[0] = %+v, want literal 'cat'", tokens[0])
	}
	if tokens[1].Literal {
		t.Errorf("token[1] should be variable (path), got literal")
	}
}

func TestTokenizeBashCommand_UUIDBecomesVar(t *testing.T) {
	tokens := TokenizeBashCommand("kubectl delete pod abc12345-def6-7890-abcd-ef1234567890")
	found := false
	for _, tok := range tokens {
		if !tok.Literal && tok.Value == "abc12345-def6-7890-abcd-ef1234567890" {
			found = true
		}
	}
	if !found {
		t.Error("UUID should be detected as variable token")
	}
}

func TestTokenizeBashCommand_NamespaceFlag(t *testing.T) {
	tokens := TokenizeBashCommand("kubectl get pods -n staging")
	for i, tok := range tokens {
		if tok.Value == "-n" && !tok.Literal {
			t.Errorf("flag -n should be literal")
		}
		if i > 0 && tokens[i-1].Value == "-n" && tok.Literal {
			t.Errorf("value after -n should be variable, got literal")
		}
	}
}

func TestTokenizeBashCommand_IPBecomesVar(t *testing.T) {
	tokens := TokenizeBashCommand("ping 192.168.1.100")
	found := false
	for _, tok := range tokens {
		if !tok.Literal && tok.Value == "192.168.1.100" {
			found = true
		}
	}
	if !found {
		t.Error("IP address should be detected as variable token")
	}
}

func TestTokenizeBashCommand_FlagEqualsValue(t *testing.T) {
	tokens := TokenizeBashCommand("kubectl get pods --namespace=staging")
	if len(tokens) != 5 {
		t.Fatalf("got %d tokens, want 5: %+v", len(tokens), tokens)
	}
	// --namespace=staging should be split into flag (literal) + value (variable)
	if tokens[3].Value != "--namespace" || !tokens[3].Literal {
		t.Errorf("token[3] = %+v, want literal '--namespace'", tokens[3])
	}
	if tokens[4].Value != "staging" || tokens[4].Literal {
		t.Errorf("token[4] = %+v, want variable 'staging'", tokens[4])
	}
}

func TestInputShapeBash(t *testing.T) {
	input := map[string]interface{}{
		"command": "kubectl get pods -n staging",
	}
	shape := InputShape("Bash", input)
	if shape["command"] != "kubectl get pods -n {VAR}" {
		t.Errorf("shape[command] = %q", shape["command"])
	}
}

func TestInputShapeBash_DifferentNamespace(t *testing.T) {
	shape1 := InputShape("Bash", map[string]interface{}{"command": "kubectl get pods -n staging"})
	shape2 := InputShape("Bash", map[string]interface{}{"command": "kubectl get pods -n production"})
	if shape1["command"] != shape2["command"] {
		t.Errorf("same structure should produce same shape: %q vs %q", shape1["command"], shape2["command"])
	}
}

func TestInputShapeNonBash(t *testing.T) {
	input := map[string]interface{}{
		"file_path": "/some/path.go",
		"line":      42,
	}
	shape := InputShape("Read", input)
	if shape["file_path"] != "{STRING}" {
		t.Errorf("shape[file_path] = %q, want {STRING}", shape["file_path"])
	}
	if shape["line"] != "{NUMBER}" {
		t.Errorf("shape[line] = %q, want {NUMBER}", shape["line"])
	}
}

func TestNodeID_SameShapeSameID(t *testing.T) {
	shape1 := InputShape("Bash", map[string]interface{}{"command": "kubectl get pods -n staging"})
	shape2 := InputShape("Bash", map[string]interface{}{"command": "kubectl get pods -n production"})
	id1 := NodeID("Bash", shape1)
	id2 := NodeID("Bash", shape2)
	if id1 != id2 {
		t.Errorf("same tool+shape should produce same NodeID: %d vs %d", id1, id2)
	}
}

func TestNodeID_DifferentToolDifferentID(t *testing.T) {
	shape := map[string]string{"command": "ls"}
	id1 := NodeID("Bash", shape)
	id2 := NodeID("Write", shape)
	if id1 == id2 {
		t.Error("different tools should produce different NodeIDs")
	}
}
