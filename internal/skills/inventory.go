package skills

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// SkillMeta holds metadata for a compiled skill.
type SkillMeta struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	GeneratedBy string `json:"generated_by"`
	Version     int    `json:"version"`
	Scope       string `json:"scope"`
	Path        string `json:"path"`
	RawContent  string `json:"raw_content"`
}

// ScanSkillsDir reads all skill directories under the given root and returns metadata.
func ScanSkillsDir(root string) ([]SkillMeta, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var skills []SkillMeta

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillPath := filepath.Join(root, entry.Name(), "SKILL.md")
		data, err := os.ReadFile(skillPath)
		if err != nil {
			continue
		}

		meta := parseSkillFrontmatter(string(data))
		meta.Path = filepath.Join(root, entry.Name())
		meta.RawContent = string(data)

		// Merge AJ-specific metadata from metadata.json
		metadataPath := filepath.Join(root, entry.Name(), "metadata.json")
		if mjson, err := os.ReadFile(metadataPath); err == nil {
			var ajMeta struct {
				GeneratedBy string `json:"generated_by"`
				Version     int    `json:"version"`
				Scope       string `json:"scope"`
			}
			if json.Unmarshal(mjson, &ajMeta) == nil {
				meta.GeneratedBy = ajMeta.GeneratedBy
				meta.Version = ajMeta.Version
				meta.Scope = ajMeta.Scope
			}
		}

		if meta.Name == "" {
			meta.Name = entry.Name()
		}

		skills = append(skills, meta)
	}

	return skills, nil
}

// parseSkillFrontmatter extracts only standard Claude Code fields (name, description).
func parseSkillFrontmatter(content string) SkillMeta {
	var meta SkillMeta

	scanner := bufio.NewScanner(strings.NewReader(content))
	inFrontmatter := false

	for scanner.Scan() {
		line := scanner.Text()

		if line == "---" {
			if !inFrontmatter {
				inFrontmatter = true
				continue
			}
			break // end of frontmatter
		}

		if !inFrontmatter {
			continue
		}

		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		switch key {
		case "name":
			meta.Name = val
		case "description":
			meta.Description = val
		}
	}

	return meta
}
