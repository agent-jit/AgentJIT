package skills

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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

		skillPath := filepath.Join(dir, entry.Name(), "skill.md")
		data, err := os.ReadFile(skillPath)
		if err != nil {
			continue
		}

		info := parseSkillInfo(string(data))
		info.Path = filepath.Join(dir, entry.Name())
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

func parseSkillInfo(content string) SkillInfo {
	var info SkillInfo
	inFrontmatter := false
	inROI := false

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

		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "roi:") {
			inROI = true
			continue
		}

		if inROI && !strings.HasPrefix(line, "  ") && !strings.HasPrefix(line, "\t") {
			inROI = false
		}

		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		val := strings.TrimSpace(parts[1])

		if inROI {
			switch key {
			case "savings_per_invocation":
				info.SavingsPerInvocation, _ = strconv.Atoi(val)
			case "observed_frequency":
				info.ObservedFrequency, _ = strconv.Atoi(val)
			case "total_projected_savings":
				info.TotalSavings, _ = strconv.Atoi(val)
			}
		} else {
			switch key {
			case "name":
				info.Name = val
			case "description":
				info.Description = val
			case "scope":
				info.Scope = val
			case "version":
				info.Version = val
			}
		}
	}

	return info
}
