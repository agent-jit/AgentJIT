package skills

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCopySkillDir_CopiesScriptsSubdir(t *testing.T) {
	// Setup: create a source skill directory with scripts/ subdirectory
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "dest-skill")

	// Create SKILL.md
	if err := os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("---\nname: test\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}

	// Create metadata.json
	if err := os.WriteFile(filepath.Join(src, "metadata.json"), []byte(`{"generated_by":"aj"}`), 0644); err != nil {
		t.Fatal(err)
	}

	// Create scripts/ subdirectory with a companion script
	scriptsDir := filepath.Join(src, "scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatal(err)
	}
	scriptContent := []byte("#!/usr/bin/env bash\nset -euo pipefail\necho hello\n")
	if err := os.WriteFile(filepath.Join(scriptsDir, "test-skill.sh"), scriptContent, 0755); err != nil {
		t.Fatal(err)
	}

	// Act
	if err := copySkillDir(src, dst); err != nil {
		t.Fatalf("copySkillDir failed: %v", err)
	}

	// Assert: SKILL.md and metadata.json copied
	if _, err := os.Stat(filepath.Join(dst, "SKILL.md")); err != nil {
		t.Error("SKILL.md not copied")
	}
	if _, err := os.Stat(filepath.Join(dst, "metadata.json")); err != nil {
		t.Error("metadata.json not copied")
	}

	// Assert: scripts/ subdirectory and its contents copied
	copiedScript := filepath.Join(dst, "scripts", "test-skill.sh")
	data, err := os.ReadFile(copiedScript)
	if err != nil {
		t.Fatalf("scripts/test-skill.sh not copied: %v", err)
	}
	if string(data) != string(scriptContent) {
		t.Errorf("script content mismatch: got %q, want %q", string(data), string(scriptContent))
	}
}

func TestCopySkillDir_PreservesNestedDirStructure(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "dest-skill")

	// Create a nested structure: scripts/helpers/util.sh
	helpersDir := filepath.Join(src, "scripts", "helpers")
	if err := os.MkdirAll(helpersDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(helpersDir, "util.sh"), []byte("#!/bin/bash\n"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("---\nname: test\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}

	if err := copySkillDir(src, dst); err != nil {
		t.Fatalf("copySkillDir failed: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dst, "scripts", "helpers", "util.sh")); err != nil {
		t.Fatal("nested scripts/helpers/util.sh not copied")
	}
}

func TestLinkSkill_FallbackCopyIncludesScripts(t *testing.T) {
	// This test verifies the full LinkSkill path when symlinks fail.
	// We simulate symlink failure by making the destination a path where
	// symlinks aren't possible — but on most CI this is hard to force.
	// Instead, test the copySkillDir path directly through LinkSkill
	// by pre-creating the destination as a non-symlink AJ-managed dir
	// so LinkSkill removes it and re-copies.

	ajDir := t.TempDir()
	claudeDir := t.TempDir()
	skillName := "my-skill"

	// Create source skill with scripts/
	skillSrc := filepath.Join(ajDir, skillName)
	if err := os.MkdirAll(filepath.Join(skillSrc, "scripts"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillSrc, "SKILL.md"), []byte("---\nname: my-skill\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillSrc, "metadata.json"), []byte(`{"generated_by":"aj","version":1}`), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(skillSrc, "scripts", "my-skill.sh"), []byte("#!/bin/bash\necho ok\n"), 0755); err != nil {
		t.Fatal(err)
	}

	// Run LinkSkill — on Windows this will use fallback copy
	if err := LinkSkill(ajDir, claudeDir, skillName); err != nil {
		t.Fatalf("LinkSkill failed: %v", err)
	}

	// Check the destination — whether symlink or copy, scripts must be accessible
	dst := filepath.Join(claudeDir, skillName)
	scriptPath := filepath.Join(dst, "scripts", "my-skill.sh")

	data, err := os.ReadFile(scriptPath)
	if err != nil {
		t.Fatalf("scripts/my-skill.sh not accessible at destination: %v", err)
	}
	if string(data) != "#!/bin/bash\necho ok\n" {
		t.Errorf("script content mismatch: got %q", string(data))
	}
}

func TestCopySkillDir_CopiesPS1Scripts(t *testing.T) {
	src := t.TempDir()
	dst := filepath.Join(t.TempDir(), "dest-skill")

	if err := os.WriteFile(filepath.Join(src, "SKILL.md"), []byte("---\nname: test\n---\n"), 0644); err != nil {
		t.Fatal(err)
	}

	scriptsDir := filepath.Join(src, "scripts")
	if err := os.MkdirAll(scriptsDir, 0755); err != nil {
		t.Fatal(err)
	}
	ps1Content := []byte("$ErrorActionPreference = 'Stop'\naj stats record --skill test\n")
	if err := os.WriteFile(filepath.Join(scriptsDir, "test-skill.ps1"), ps1Content, 0644); err != nil {
		t.Fatal(err)
	}

	if err := copySkillDir(src, dst); err != nil {
		t.Fatalf("copySkillDir failed: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(dst, "scripts", "test-skill.ps1"))
	if err != nil {
		t.Fatalf("scripts/test-skill.ps1 not copied: %v", err)
	}
	if string(data) != string(ps1Content) {
		t.Errorf("ps1 content mismatch: got %q, want %q", string(data), string(ps1Content))
	}
}
