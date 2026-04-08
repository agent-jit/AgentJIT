package skills

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// LinkSkill creates a symlink from the Claude Code skills directory to the AJ
// skill directory, making the skill discoverable by Claude Code.
// On symlink failure (e.g. Windows without developer mode), falls back to
// copying the SKILL.md file.
func LinkSkill(ajSkillsDir, claudeSkillsDir, skillName string) error {
	src := filepath.Join(ajSkillsDir, skillName)
	dst := filepath.Join(claudeSkillsDir, skillName)

	// Skip if source doesn't exist
	if _, err := os.Stat(src); os.IsNotExist(err) {
		return fmt.Errorf("skill %q not found in %s", skillName, ajSkillsDir)
	}

	// Ensure Claude skills directory exists
	if err := os.MkdirAll(claudeSkillsDir, 0755); err != nil {
		return fmt.Errorf("creating claude skills dir: %w", err)
	}

	// Remove existing link/dir at destination
	if info, err := os.Lstat(dst); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			os.Remove(dst)
		} else if info.IsDir() {
			// Check if it's an AJ-managed copy (has metadata.json with generated_by: aj)
			if isAJManaged(dst) {
				os.RemoveAll(dst)
			} else {
				return nil // Don't overwrite a non-AJ skill
			}
		}
	}

	// Try symlink first
	if err := os.Symlink(src, dst); err == nil {
		return nil
	}

	// Fallback: copy the skill directory
	return copySkillDir(src, dst)
}

// UnlinkSkill removes the symlink or copied skill from the Claude Code skills directory.
func UnlinkSkill(claudeSkillsDir, skillName string) error {
	dst := filepath.Join(claudeSkillsDir, skillName)

	info, err := os.Lstat(dst)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	// Only remove if it's a symlink or AJ-managed copy
	if info.Mode()&os.ModeSymlink != 0 || isAJManaged(dst) {
		return os.RemoveAll(dst)
	}

	return nil
}

// SyncLinks ensures all AJ skills have corresponding entries in the Claude
// Code skills directory.
func SyncLinks(ajSkillsDir, claudeSkillsDir string) error {
	entries, err := os.ReadDir(ajSkillsDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		// Only link if SKILL.md exists
		skillFile := filepath.Join(ajSkillsDir, entry.Name(), "SKILL.md")
		if _, err := os.Stat(skillFile); os.IsNotExist(err) {
			continue
		}
		if err := LinkSkill(ajSkillsDir, claudeSkillsDir, entry.Name()); err != nil {
			// Log but don't fail on individual link errors
			fmt.Printf("[AJ] Warning: could not link skill %s: %v\n", entry.Name(), err)
		}
	}

	return nil
}

// isAJManaged checks if a skill directory was created/managed by AJ
// by looking for metadata.json with generated_by: "aj".
func isAJManaged(dir string) bool {
	data, err := os.ReadFile(filepath.Join(dir, "metadata.json"))
	if err != nil {
		return false
	}
	// Simple check — avoid importing encoding/json for a small helper
	return len(data) > 0 && strings.Contains(string(data), `"generated_by"`) && strings.Contains(string(data), `"aj"`)
}

// copySkillDir recursively copies a skill directory (SKILL.md, metadata.json, scripts/).
func copySkillDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copySkillDir(srcPath, dstPath); err != nil {
				return err
			}
			continue
		}

		data, err := os.ReadFile(srcPath)
		if err != nil {
			continue
		}

		info, _ := entry.Info()
		perm := os.FileMode(0644)
		if info != nil {
			perm = info.Mode().Perm()
		}
		if err := os.WriteFile(dstPath, data, perm); err != nil {
			return err
		}
	}

	return nil
}
