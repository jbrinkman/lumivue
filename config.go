package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// Source represents a configured camera or RTSP stream source.
type Source struct {
	ID        string `json:"id"`
	Type      string `json:"type"` // "usb" or "rtsp"
	Name      string `json:"name"`
	URL       string `json:"url,omitempty"`
	IsDefault bool   `json:"isDefault"`
}

// Config is the root configuration persisted to disk.
type Config struct {
	Sources          []Source `json:"sources"`
	LastMonitorIndex int      `json:"lastMonitorIndex"`
}

// ConfigManager handles reading and writing config to disk.
type ConfigManager struct {
	mu   sync.RWMutex
	path string
	cfg  Config
}

// NewConfigManager creates a ConfigManager, loading any existing config from disk.
func NewConfigManager() (*ConfigManager, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return nil, fmt.Errorf("resolve user config dir: %w", err)
	}
	appDir := filepath.Join(dir, "lumivue")
	if err := os.MkdirAll(appDir, 0o700); err != nil {
		return nil, fmt.Errorf("create config dir: %w", err)
	}
	path := filepath.Join(appDir, "config.json")

	cm := &ConfigManager{path: path}
	if err := cm.load(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("load config: %w", err)
	}
	return cm, nil
}

// load reads config from disk into cm.cfg. Returns os.ErrNotExist if no file yet.
func (cm *ConfigManager) load() error {
	data, err := os.ReadFile(cm.path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, &cm.cfg)
}

// Load returns a copy of the current config.
func (cm *ConfigManager) Load() Config {
	cm.mu.RLock()
	defer cm.mu.RUnlock()
	return cm.cfg
}

// Save persists the given config to disk and updates the in-memory copy.
func (cm *ConfigManager) Save(cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	// Write to a temp file then rename for atomic update.
	tmp := cm.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return fmt.Errorf("write config tmp: %w", err)
	}
	if err := os.Rename(tmp, cm.path); err != nil {
		return fmt.Errorf("rename config: %w", err)
	}
	cm.mu.Lock()
	cm.cfg = cfg
	cm.mu.Unlock()
	return nil
}
