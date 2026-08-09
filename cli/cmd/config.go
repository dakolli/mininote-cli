package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Config is the CLI's persisted session state on disk.
type Config struct {
	BaseURL      string `json:"baseURL"`
	Token        string `json:"token"`
	RefreshToken string `json:"refreshToken"`
	Subject      string `json:"subject"`
	ExpiresAt    string `json:"expiresAt"`
}

// Load reads the config file. A missing file yields an empty config, not an
// error.
func Load(path string) (*Config, error) {
	cfg := &Config{}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil
		}
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	return cfg, nil
}

// Save writes the config file, creating its parent directory, with 0600 perms.
func Save(path string, cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("write config %s: %w", path, err)
	}
	return nil
}
