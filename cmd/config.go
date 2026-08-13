package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config is the CLI's persisted session state on disk.
type Config struct {
	BaseURL      string `json:"base_url" yaml:"base_url,omitempty"`
	Token        string `json:"token" yaml:"token,omitempty"`
	RefreshToken string `json:"refresh_token" yaml:"refresh_token,omitempty"`
	Subject      string `json:"subject" yaml:"subject,omitempty"`
	ExpiresAt    string `json:"expires_at" yaml:"expires_at,omitempty"`
}

// Load reads the config file. A missing file yields an empty config, not an
// error.
func Load(path string) (*Config, error) {
	cfg := &Config{}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Fallback: check if legacy cli.json exists when looking for cli.yaml
			if filepath.Base(path) == "cli.yaml" {
				legacyPath := filepath.Join(filepath.Dir(path), "cli.json")
				if legacyData, legacyErr := os.ReadFile(legacyPath); legacyErr == nil {
					_ = yaml.Unmarshal(legacyData, cfg)
					return cfg, nil
				}
			}
			return cfg, nil
		}
		return nil, fmt.Errorf("read config %s: %w", path, err)
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("parse config %s: %w", path, err)
	}
	return cfg, nil
}

// Save writes the config file as YAML, creating its parent directory, with 0600 perms.
func Save(path string, cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("write config %s: %w", path, err)
	}
	return nil
}
