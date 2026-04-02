package hooks

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// InstallHooks merges AJ hooks into the given Claude Code settings file.
// Creates the file if it doesn't exist. Preserves existing hooks and settings.
// Idempotent -- replaces any existing AJ/legacy hooks with current versions.
func InstallHooks(settingsPath string) error {
	// First remove any existing AJ or legacy hooks
	_ = UninstallHooks(settingsPath)

	settings, err := readSettings(settingsPath)
	if err != nil {
		return err
	}

	hooksObj, _ := settings["hooks"].(map[string]interface{})
	if hooksObj == nil {
		hooksObj = make(map[string]interface{})
	}

	ajHooks := AJHooks()

	for event, groups := range ajHooks {
		existing, _ := hooksObj[event].([]interface{})

		// Marshal our groups and append
		for _, group := range groups {
			groupJSON, _ := json.Marshal(group)
			var groupMap interface{}
			_ = json.Unmarshal(groupJSON, &groupMap)
			existing = append(existing, groupMap)
		}
		hooksObj[event] = existing
	}

	settings["hooks"] = hooksObj
	return writeSettings(settingsPath, settings)
}

// UninstallHooks removes AJ hooks from the given Claude Code settings file.
// Preserves all non-AJ hooks and settings.
func UninstallHooks(settingsPath string) error {
	settings, err := readSettings(settingsPath)
	if err != nil {
		return err
	}

	hooksObj, _ := settings["hooks"].(map[string]interface{})
	if hooksObj == nil {
		return nil
	}

	for event, val := range hooksObj {
		groups, ok := val.([]interface{})
		if !ok {
			continue
		}

		var kept []interface{}
		for _, g := range groups {
			groupMap, ok := g.(map[string]interface{})
			if !ok {
				kept = append(kept, g)
				continue
			}
			handlers, ok := groupMap["hooks"].([]interface{})
			if !ok {
				kept = append(kept, g)
				continue
			}

			hasAJ := false
			for _, h := range handlers {
				hm, ok := h.(map[string]interface{})
				if !ok {
					continue
				}
				cmd, _ := hm["command"].(string)
				if isAJHook(cmd) {
					hasAJ = true
					break
				}
			}
			if !hasAJ {
				kept = append(kept, g)
			}
		}

		if len(kept) == 0 {
			delete(hooksObj, event)
		} else {
			hooksObj[event] = kept
		}
	}

	if len(hooksObj) == 0 {
		delete(settings, "hooks")
	} else {
		settings["hooks"] = hooksObj
	}

	return writeSettings(settingsPath, settings)
}


func readSettings(path string) (map[string]interface{}, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return make(map[string]interface{}), nil
		}
		return nil, fmt.Errorf("reading settings: %w", err)
	}

	var settings map[string]interface{}
	if err := json.Unmarshal(data, &settings); err != nil {
		return nil, fmt.Errorf("parsing settings: %w", err)
	}
	return settings, nil
}

func writeSettings(path string, settings map[string]interface{}) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("creating settings dir: %w", err)
	}

	data, err := json.MarshalIndent(settings, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling settings: %w", err)
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0644)
}
