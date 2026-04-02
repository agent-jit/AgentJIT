package config

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
)

// GetField retrieves a value from Config by dot-notation key (e.g. "compile.trigger_mode").
func GetField(cfg Config, key string) (interface{}, error) {
	data, err := json.Marshal(cfg)
	if err != nil {
		return nil, err
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}

	parts := strings.SplitN(key, ".", 2)
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid key %q: must be section.field (e.g. compile.trigger_mode)", key)
	}

	section, ok := m[parts[0]]
	if !ok {
		return nil, fmt.Errorf("unknown section %q", parts[0])
	}

	sectionMap, ok := section.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("section %q is not an object", parts[0])
	}

	val, ok := sectionMap[parts[1]]
	if !ok {
		return nil, fmt.Errorf("unknown key %q in section %q", parts[1], parts[0])
	}

	return val, nil
}

// SetField updates a value in Config by dot-notation key. Values are auto-typed
// (numbers become float64, "true"/"false" become bool, everything else stays string).
func SetField(cfg Config, key, value string) (Config, error) {
	data, err := json.Marshal(cfg)
	if err != nil {
		return cfg, err
	}

	var m map[string]interface{}
	if err := json.Unmarshal(data, &m); err != nil {
		return cfg, err
	}

	parts := strings.SplitN(key, ".", 2)
	if len(parts) != 2 {
		return cfg, fmt.Errorf("invalid key %q: must be section.field", key)
	}

	section, ok := m[parts[0]]
	if !ok {
		return cfg, fmt.Errorf("unknown section %q", parts[0])
	}

	sectionMap, ok := section.(map[string]interface{})
	if !ok {
		return cfg, fmt.Errorf("section %q is not an object", parts[0])
	}

	if _, ok := sectionMap[parts[1]]; !ok {
		return cfg, fmt.Errorf("unknown key %q in section %q", parts[1], parts[0])
	}

	// Auto-type the value
	var typed interface{}
	if n, err := strconv.ParseFloat(value, 64); err == nil {
		typed = n
	} else if b, err := strconv.ParseBool(value); err == nil {
		typed = b
	} else {
		typed = value
	}

	sectionMap[parts[1]] = typed
	m[parts[0]] = sectionMap

	updated, err := json.Marshal(m)
	if err != nil {
		return cfg, err
	}

	var result Config
	if err := json.Unmarshal(updated, &result); err != nil {
		return cfg, err
	}

	return result, nil
}
