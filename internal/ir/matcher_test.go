package ir

import (
	"testing"
)

func testCatalog() *Catalog {
	return &Catalog{
		Version: 1,
		Domains: map[string]map[string]Capability{
			"git": {
				"GIT_STATUS":   {Verbs: []string{"git status"}, Params: []string{}},
				"GIT_COMMIT":   {Verbs: []string{"git commit"}, Params: []string{"MESSAGE", "FLAGS"}},
				"GIT_CHECKOUT": {Verbs: []string{"git checkout", "git switch"}, Params: []string{"BRANCH"}},
			},
			"k8s": {
				"K8S_LOG":      {Verbs: []string{"kubectl logs", "stern"}, Params: []string{"POD", "NAMESPACE", "CONTAINER"}},
				"K8S_GET_PODS": {Verbs: []string{"kubectl get pods", "kubectl get po"}, Params: []string{"NAMESPACE", "LABELS"}},
			},
			"fs": {
				"FS_READ": {Verbs: []string{"cat", "head", "tail"}, Params: []string{"PATH"}},
			},
		},
	}
}

func TestMatcher_ExactVerbMatch(t *testing.T) {
	m := NewMatcher(testCatalog())
	match := m.Match("git status")
	if match == nil {
		t.Fatal("expected match for 'git status'")
	}
	if match.CapabilityID != "GIT_STATUS" {
		t.Errorf("got %s, want GIT_STATUS", match.CapabilityID)
	}
}

func TestMatcher_VerbWithArgs(t *testing.T) {
	m := NewMatcher(testCatalog())
	match := m.Match("kubectl logs my-pod -n prod")
	if match == nil {
		t.Fatal("expected match for 'kubectl logs'")
	}
	if match.CapabilityID != "K8S_LOG" {
		t.Errorf("got %s, want K8S_LOG", match.CapabilityID)
	}
}

func TestMatcher_LongestPrefixWins(t *testing.T) {
	m := NewMatcher(testCatalog())
	match := m.Match("kubectl get pods -n staging")
	if match == nil {
		t.Fatal("expected match")
	}
	if match.CapabilityID != "K8S_GET_PODS" {
		t.Errorf("got %s, want K8S_GET_PODS", match.CapabilityID)
	}
}

func TestMatcher_AlternateVerb(t *testing.T) {
	m := NewMatcher(testCatalog())
	match := m.Match("git switch feature-branch")
	if match == nil {
		t.Fatal("expected match")
	}
	if match.CapabilityID != "GIT_CHECKOUT" {
		t.Errorf("got %s, want GIT_CHECKOUT", match.CapabilityID)
	}
}

func TestMatcher_CrossToolCollapse(t *testing.T) {
	m := NewMatcher(testCatalog())
	m1 := m.Match("kubectl logs my-pod -n prod")
	m2 := m.Match("stern other-pod -n staging")
	if m1 == nil || m2 == nil {
		t.Fatal("expected both to match")
	}
	if m1.CapabilityID != m2.CapabilityID {
		t.Errorf("cross-tool mismatch: %s vs %s", m1.CapabilityID, m2.CapabilityID)
	}
}

func TestMatcher_NoMatch(t *testing.T) {
	m := NewMatcher(testCatalog())
	match := m.Match("terraform plan")
	if match != nil {
		t.Errorf("expected no match for 'terraform plan', got %s", match.CapabilityID)
	}
}

func TestMatcher_SingleWordVerb(t *testing.T) {
	m := NewMatcher(testCatalog())
	match := m.Match("cat /etc/hosts")
	if match == nil {
		t.Fatal("expected match for 'cat'")
	}
	if match.CapabilityID != "FS_READ" {
		t.Errorf("got %s, want FS_READ", match.CapabilityID)
	}
}

func TestMatcher_ParamShape(t *testing.T) {
	m := NewMatcher(testCatalog())
	m1 := m.Match("kubectl logs pod-a -n prod")
	m2 := m.Match("kubectl logs pod-b -n staging")
	if m1 == nil || m2 == nil {
		t.Fatal("expected both to match")
	}
	if m1.ParamShape() != m2.ParamShape() {
		t.Errorf("same command with different values should have same param shape: %q vs %q",
			m1.ParamShape(), m2.ParamShape())
	}
}
