package ir

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadCatalog_FromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ir_catalog.yaml")
	content := `version: 1
domains:
  git:
    GIT_STATUS:
      verbs: ["git status"]
      params: []
    GIT_COMMIT:
      verbs: ["git commit"]
      params: [MESSAGE, FLAGS]
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cat, err := LoadCatalog(path)
	if err != nil {
		t.Fatal(err)
	}
	if cat.Version != 1 {
		t.Errorf("version = %d, want 1", cat.Version)
	}
	if len(cat.Domains) != 1 {
		t.Fatalf("got %d domains, want 1", len(cat.Domains))
	}
	gitDomain := cat.Domains["git"]
	if len(gitDomain) != 2 {
		t.Fatalf("got %d capabilities in git, want 2", len(gitDomain))
	}
	status := gitDomain["GIT_STATUS"]
	if len(status.Verbs) != 1 || status.Verbs[0] != "git status" {
		t.Errorf("GIT_STATUS verbs = %v", status.Verbs)
	}
	commit := gitDomain["GIT_COMMIT"]
	if len(commit.Params) != 2 || commit.Params[0] != "MESSAGE" {
		t.Errorf("GIT_COMMIT params = %v", commit.Params)
	}
}

func TestLoadCatalog_FileNotFound(t *testing.T) {
	_, err := LoadCatalog("/nonexistent/path.yaml")
	if err == nil {
		t.Error("expected error for missing file")
	}
}

func TestCatalog_AllEntries(t *testing.T) {
	cat := Catalog{
		Version: 1,
		Domains: map[string]map[string]Capability{
			"git": {
				"GIT_STATUS": {Verbs: []string{"git status"}, Params: []string{}},
			},
			"k8s": {
				"K8S_LOG": {Verbs: []string{"kubectl logs", "stern"}, Params: []string{"POD"}},
			},
		},
	}
	entries := cat.AllEntries()
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
}
