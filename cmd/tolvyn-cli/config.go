package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Config struct {
	APIURL             string `json:"api_url"`
	Token              string `json:"token,omitempty"`
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

func saveConfig(cfg *Config) error {
	path := configPath()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}
