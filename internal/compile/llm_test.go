// internal/compile/llm_test.go
package compile

import "testing"

func TestLLMBackend_Name(t *testing.T) {
	b := NewLLMBackend(LLMBackendConfig{})
	if b.Name() != "aj-llm" {
		t.Errorf("Name() = %q, want aj-llm", b.Name())
	}
}
