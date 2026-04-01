package config

import (
	"encoding/json"
	"os"
)

type DaemonConfig struct {
	IdleTimeoutMinutes int    `json:"idle_timeout_minutes"`
	SocketPath         string `json:"socket_path,omitempty"`
}

type IngestionConfig struct {
	MaxResponseBytes int `json:"max_response_bytes"`
	LogRetentionDays int `json:"log_retention_days"`
}

type DreamConfig struct {
	TriggerMode           string `json:"trigger_mode"`
	TriggerIntervalMinutes int   `json:"trigger_interval_minutes"`
	TriggerEventThreshold  int   `json:"trigger_event_threshold"`
	MaxContextLines        int   `json:"max_context_lines"`
	MinPatternFrequency    int   `json:"min_pattern_frequency"`
	MinTokenSavings        int   `json:"min_token_savings"`
	DeprecateAfterSessions int   `json:"deprecate_after_sessions"`
}

type ScopeConfig struct {
	GlobalCLITools        []string `json:"global_cli_tools"`
	CrossProjectThreshold int      `json:"cross_project_threshold"`
}

type Config struct {
	Daemon    DaemonConfig    `json:"daemon"`
	Ingestion IngestionConfig `json:"ingestion"`
	Dream     DreamConfig     `json:"dream"`
	Scope     ScopeConfig     `json:"scope"`
}

func DefaultConfig() Config {
	return Config{
		Daemon: DaemonConfig{
			IdleTimeoutMinutes: 30,
		},
		Ingestion: IngestionConfig{
			MaxResponseBytes: 512,
			LogRetentionDays: 30,
		},
		Dream: DreamConfig{
			TriggerMode:            "manual",
			TriggerIntervalMinutes: 30,
			TriggerEventThreshold:  100,
			MaxContextLines:        50000,
			MinPatternFrequency:    3,
			MinTokenSavings:        500,
			DeprecateAfterSessions: 20,
		},
		Scope: ScopeConfig{
			GlobalCLITools: []string{
				"kubectl", "az", "gh", "docker", "aws",
				"gcloud", "terraform", "helm", "ssh", "scp",
			},
			CrossProjectThreshold: 2,
		},
	}
}

// Load reads config from the given path. Returns default config if file doesn't exist.
func Load(path string) (Config, error) {
	cfg := DefaultConfig()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return cfg, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cfg, err
	}
	return cfg, nil
}

// Save writes config to the given path.
func Save(path string, cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
