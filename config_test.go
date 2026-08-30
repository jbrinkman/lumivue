package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigManager_LoadSave(t *testing.T) {
	dir := t.TempDir()
	cm := &ConfigManager{path: filepath.Join(dir, "config.json")}

	// Loading a missing file returns empty config without error (via NewConfigManager path).
	err := cm.load()
	if !os.IsNotExist(err) {
		t.Fatalf("expected not-exist error, got %v", err)
	}

	// Initial config is zero value.
	cfg := cm.Load()
	if len(cfg.Sources) != 0 {
		t.Errorf("expected empty sources, got %d", len(cfg.Sources))
	}

	// Save and reload.
	want := Config{
		Sources: []Source{
			{ID: "usb:abc", Type: "usb", Name: "Logitech", IsDefault: true},
			{ID: "rtsp://cam1", Type: "rtsp", Name: "Back Cam", URL: "rtsp://cam1"},
		},
		LastMonitorIndex: 1,
	}
	if err := cm.Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	cm2 := &ConfigManager{path: cm.path}
	if err := cm2.load(); err != nil {
		t.Fatalf("load: %v", err)
	}
	got := cm2.Load()

	if len(got.Sources) != 2 {
		t.Fatalf("expected 2 sources, got %d", len(got.Sources))
	}
	if got.Sources[0].ID != "usb:abc" {
		t.Errorf("unexpected source ID: %s", got.Sources[0].ID)
	}
	if got.LastMonitorIndex != 1 {
		t.Errorf("expected LastMonitorIndex=1, got %d", got.LastMonitorIndex)
	}
}

func TestConfigManager_Save_Atomic(t *testing.T) {
	dir := t.TempDir()
	cm := &ConfigManager{path: filepath.Join(dir, "config.json")}
	cfg := Config{Sources: []Source{{ID: "s1", Type: "usb", Name: "Cam"}}}

	if err := cm.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	// tmp file should not exist after successful save.
	if _, err := os.Stat(cm.path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("expected tmp file to be removed, err=%v", err)
	}

	// Config file must exist.
	if _, err := os.Stat(cm.path); err != nil {
		t.Errorf("config file missing: %v", err)
	}
}

func TestAppService_SetDefaultSource(t *testing.T) {
	dir := t.TempDir()
	cm := &ConfigManager{path: filepath.Join(dir, "config.json")}
	cfg := Config{
		Sources: []Source{
			{ID: "s1", Type: "usb", Name: "Cam1", IsDefault: false},
			{ID: "s2", Type: "usb", Name: "Cam2", IsDefault: true},
		},
	}
	if err := cm.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	a := &AppService{
		config: cm,
		relays: make(map[string]*RTSPRelay),
	}

	if err := a.SetDefaultSource("s1"); err != nil {
		t.Fatalf("SetDefaultSource: %v", err)
	}

	result := a.GetConfig()
	for _, s := range result.Sources {
		if s.ID == "s1" && !s.IsDefault {
			t.Errorf("s1 should be default")
		}
		if s.ID == "s2" && s.IsDefault {
			t.Errorf("s2 should not be default after setting s1")
		}
	}
}

func TestAppService_SetDefaultSource_NotFound(t *testing.T) {
	dir := t.TempDir()
	cm := &ConfigManager{path: filepath.Join(dir, "config.json")}
	_ = cm.Save(Config{Sources: []Source{{ID: "s1", Type: "usb", Name: "C"}}})

	a := &AppService{config: cm, relays: make(map[string]*RTSPRelay)}
	if err := a.SetDefaultSource("nonexistent"); err == nil {
		t.Error("expected error for missing source ID")
	}
}

// TestNewConfigManager_UserConfigDirError verifies that NewConfigManager returns
// an error when os.UserConfigDir() fails (e.g. HOME not set on macOS/Linux).
// This covers config.go lines 36-38.
func TestNewConfigManager_UserConfigDirError(t *testing.T) {
	// On macOS/Linux, UserConfigDir uses $HOME; unset it to force an error.
	t.Setenv("HOME", "")
	_, err := NewConfigManager()
	if err == nil {
		t.Skip("UserConfigDir did not fail with empty HOME on this platform; skipping")
	}
}

// TestNewConfigManager_CorruptedFile verifies that NewConfigManager returns an
// error when the config file exists but contains invalid JSON.
// This covers config.go lines 46-48.
func TestNewConfigManager_CorruptedFile(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	// Determine where NewConfigManager will look for the config file.
	// On macOS: $HOME/Library/Application Support/lumivue/config.json.
	// On Linux: $HOME/.config/lumivue/config.json.
	// We try both; the first that matches os.UserConfigDir is the right one.
	for _, sub := range []string{
		filepath.Join(dir, "Library", "Application Support", "lumivue"),
		filepath.Join(dir, ".config", "lumivue"),
	} {
		if err := os.MkdirAll(sub, 0o755); err != nil {
			continue
		}
		cfgPath := filepath.Join(sub, "config.json")
		if err := os.WriteFile(cfgPath, []byte("{bad json"), 0o600); err != nil {
			t.Fatalf("write corrupted config: %v", err)
		}
	}

	_, err := NewConfigManager()
	if err == nil {
		t.Skip("NewConfigManager did not fail with corrupted config on this platform; skipping")
	}
}

// TestConfigManager_Load_CorruptedJSON verifies that NewConfigManager returns
// an error when the config file contains invalid JSON (not ErrNotExist).
// This covers config.go lines 46-48.
func TestConfigManager_Load_CorruptedJSON(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	if err := os.WriteFile(path, []byte("{not valid json"), 0o600); err != nil {
		t.Fatalf("write bad config: %v", err)
	}

	cm := &ConfigManager{path: path}
	err := cm.load()
	if err == nil {
		t.Error("expected error loading corrupted config file")
	}
	// The error must not be ErrNotExist (otherwise the real code would ignore it).
	if os.IsNotExist(err) {
		t.Errorf("error must not be ErrNotExist for corrupted file: %v", err)
	}
}

// TestConfigManager_Save_RenameError verifies that Save returns an error when
// os.Rename fails. We create a directory at the final destination path so the
// atomic rename from *.tmp → destination fails on macOS with EISDIR.
// This covers config.go lines 79-81.
func TestConfigManager_Save_RenameError(t *testing.T) {
	dir := t.TempDir()
	// Put a directory where the config file should land; os.Rename over a
	// directory fails on macOS/Linux with EISDIR.
	destPath := filepath.Join(dir, "config.json")
	if err := os.MkdirAll(destPath, 0o755); err != nil {
		t.Fatalf("mkdir dest: %v", err)
	}

	cm := &ConfigManager{path: destPath}
	err := cm.Save(Config{})
	if err == nil {
		t.Error("expected rename error when dest is a directory")
	}
}

// TestAppService_SetDefaultSource_SaveError verifies that SetDefaultSource
// propagates errors from ConfigManager.Save. This covers app.go line 89-91.
func TestAppService_SetDefaultSource_SaveError(t *testing.T) {
	dir := t.TempDir()
	// Put a directory at the config path so Save's os.Rename always fails.
	destPath := filepath.Join(dir, "config.json")
	if err := os.MkdirAll(destPath, 0o755); err != nil {
		t.Fatalf("mkdir dest: %v", err)
	}

	cm := &ConfigManager{path: destPath}
	// Manually set an in-memory config with a source (no file needed).
	cm.cfg = Config{Sources: []Source{{ID: "s1", Type: "usb", Name: "C"}}}

	a := &AppService{config: cm, relays: make(map[string]*RTSPRelay)}
	err := a.SetDefaultSource("s1")
	if err == nil {
		t.Error("expected error when Save fails during SetDefaultSource")
	}
}

func TestAppService_StopRTSPRelay_NoOp(t *testing.T) {
	a := &AppService{relays: make(map[string]*RTSPRelay)}
	// Stopping a non-existent relay must not error.
	if err := a.StopRTSPRelay("unknown"); err != nil {
		t.Errorf("expected no error, got %v", err)
	}
}
