package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	APIURL             string `json:"api_url"`
	Token              string `json:"token,omitempty"`
	TokenAPIURL        string `json:"token_api_url,omitempty"` // the server that issued Token (1.3.2)
	DefaultEnvironment string `json:"default_environment,omitempty"`
}

func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".tolvyn", "config.json")
}

func loadConfig() (*Config, error) {
	data, err := os.ReadFile(configPath())
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{APIURL: defaultAPIURL}, nil
		}
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.APIURL == "" {
		cfg.APIURL = defaultAPIURL
	}
	return &cfg, nil
}

// saveConfig writes the config readable only by its owner, every time: a temporary file 0600 in the same directory,
// renamed over the old one, and the directory set to 0700 (an existing config or directory kept its old mode before
// 1.3.2, BE-667).
func saveConfig(cfg *Config) error {
	path := configPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".config-*.json")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name()) //nolint:errcheck // gone after the rename
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
