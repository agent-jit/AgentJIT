package skills

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SkillInfo holds display information for a skill.
type SkillInfo struct {
	Name                 string
	Description          string
	Scope                string
	Version              string
	SavingsPerInvocation int
	ObservedFrequency    int
	TotalSavings         int
	Path                 string
}

// ListSkills scans a skills directory and returns display info for each skill.
func ListSkills(dir string) ([]SkillInfo, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var skills []SkillInfo

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		skillPath := filepath.Join(dir, entry.Name(), "SKILL.md")
		data, err := os.ReadFile(skillPath)
		if err != nil {
			continue
		}

		info := parseSkillMDInfo(string(data))
		info.Path = filepath.Join(dir, entry.Name())

		// Read AJ-specific metadata from metadata.json
		metadataPath := filepath.Join(dir, entry.Name(), "metadata.json")
		if mjson, err := os.ReadFile(metadataPath); err == nil {
			var ajMeta struct {
				Version int    `json:"version"`
				Scope   string `json:"scope"`
				ROI     struct {
					SavingsPerInvocation int `json:"savings_per_invocation"`
					ObservedFrequency    int `json:"observed_frequency"`
					TotalProjectedSavings int `json:"total_projected_savings"`
				} `json:"roi"`
			}
			if json.Unmarshal(mjson, &ajMeta) == nil {
				info.Version = fmt.Sprintf("%d", ajMeta.Version)
				info.Scope = ajMeta.Scope
				info.SavingsPerInvocation = ajMeta.ROI.SavingsPerInvocation
				info.ObservedFrequency = ajMeta.ROI.ObservedFrequency
				info.TotalSavings = ajMeta.ROI.TotalProjectedSavings
			}
		}

		if info.Name == "" {
			info.Name = entry.Name()
		}

		skills = append(skills, info)
	}

	return skills, nil
}

// RemoveSkill deletes a skill directory.
func RemoveSkill(skillsDir, name string) error {
	skillPath := filepath.Join(skillsDir, name)
	if _, err := os.Stat(skillPath); os.IsNotExist(err) {
		return fmt.Errorf("skill %q not found in %s", name, skillsDir)
	}
	return os.RemoveAll(skillPath)
}

// parseSkillMDInfo extracts name and description from SKILL.md frontmatter.
func parseSkillMDInfo(content string) SkillInfo {
	var info SkillInfo
	inFrontmatter := false

	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := scanner.Text()

		if line == "---" {
			if !inFrontmatter {
				inFrontmatter = true
				continue
			}
			break
		}

		if !inFrontmatter {
			continue
		}

		parts := strings.SplitN(strings.TrimSpace(line), ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		switch key {
		case "name":
			info.Name = val
		case "description":
			info.Description = val
		}
	}

	return info
}
