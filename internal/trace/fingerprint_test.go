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
