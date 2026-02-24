package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type Config struct {
	HomeDir         string `json:"home_dir"`
	ScheduleTime    string `json:"schedule_time"` // HH:MM
	TempArchiveDir  string `json:"temp_archive_dir"`
	ServerAddr      string `json:"server_addr"`
	APIKey          string `json:"api_key"`
	AgentID         string `json:"agent_id"`
	PollInterval    int    `json:"poll_interval_seconds"`
	APIURL          string `json:"api_url"`
	EventBufferPath string `json:"event_buffer_path"`
}

func Default() Config {
	return Config{
		HomeDir:         "",
		ScheduleTime:    "03:00",
		TempArchiveDir:  "",
		ServerAddr:      "localhost:9000",
		APIKey:          "",
		AgentID:         "",
		PollInterval:    60,
		APIURL:          "http://localhost:8080",
		EventBufferPath: "",
	}
}

func Path(appName string) (string, error) {
	baseDir, err := os.UserConfigDir()
	if err != nil || baseDir == "" {
		return "", fmt.Errorf("cannot resolve user config dir")
	}
	return filepath.Join(baseDir, appName, "config.json"), nil
}

func LoadOrCreate(appName string) (Config, string, error) {
	path, err := Path(appName)
	if err != nil {
		return Config{}, "", err
	}

	if _, err := os.Stat(path); err != nil {
		if !os.IsNotExist(err) {
			return Config{}, "", err
		}
		cfg := Default()
		if err := Save(path, cfg); err != nil {
			return Config{}, "", err
		}
		return cfg, path, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return Config{}, "", err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, "", err
	}

	return cfg, path, nil
}

func Save(path string, cfg Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}
