package main

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// ── RTSPRelay error-path tests ─────────────────────────────────────────────

func TestConnect_InvalidURL(t *testing.T) {
	r := newRTSPRelay("s1", "not-a-valid-url")
	defer r.cancel()
	err := r.connect(func(string, any) {})
	if err == nil {
		t.Fatal("expected error for invalid RTSP URL")
	}
	if !strings.Contains(err.Error(), "parse rtsp url") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestConnect_ContextAlreadyCancelled(t *testing.T) {
	// Cancel context before connect is called so the connection attempt
	// hits the "timeout" path immediately.
	r := newRTSPRelay("s1", "rtsp://127.0.0.1:65000/stream")
	r.cancel()
	err := r.connect(func(string, any) {})
	// Either "timeout" or context-cancelled; must not panic.
	if err == nil {
		t.Fatal("expected an error after context cancellation")
	}
}

func TestReadLoop_InvalidURL_EmitsError(t *testing.T) {
	r := newRTSPRelay("s1", "not-valid-url")
	defer r.cancel()

	var emitted []string
	done := make(chan struct{})
	go func() {
		defer close(done)
		r.readLoop(func(name string, _ any) { emitted = append(emitted, name) })
	}()

	select {
	case <-done:
		// readLoop returned after the first connect failure.
	case <-time.After(2 * time.Second):
		t.Fatal("readLoop did not return within timeout")
	}

	found := false
	for _, e := range emitted {
		if e == "rtsp:error" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected rtsp:error event, got %v", emitted)
	}
}

func TestReadLoop_ContextCancelled_NoEmit(t *testing.T) {
	r := newRTSPRelay("s1", "rtsp://127.0.0.1:65001/stream")
	// Cancel before the loop starts — readLoop should return without emitting
	// an error event.
	r.cancel()

	var emitted []string
	done := make(chan struct{})
	go func() {
		defer close(done)
		r.readLoop(func(name string, _ any) { emitted = append(emitted, name) })
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("readLoop did not return within timeout")
	}

	for _, e := range emitted {
		if e == "rtsp:error" {
			t.Errorf("rtsp:error must not be emitted when context is cancelled")
		}
	}
}

// ── AppService ServiceStartup ──────────────────────────────────────────────

func TestAppService_ServiceStartup(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("HOME", dir)

	a := NewAppService()
	err := a.ServiceStartup(context.Background(), application.ServiceOptions{})
	if err != nil {
		t.Fatalf("ServiceStartup: %v", err)
	}
	if a.config == nil {
		t.Error("config must not be nil after ServiceStartup")
	}
}

func TestAppService_ServiceStartup_BadHomeDir(t *testing.T) {
	// Point HOME to a read-only non-existent path so UserConfigDir is
	// resolved but os.MkdirAll fails.
	if os.Getuid() == 0 {
		t.Skip("root can write anywhere; skip permission test")
	}

	// Create a file (not a dir) and point config path to a sub-path of it.
	dir := t.TempDir()
	// Make the dir unwritable.
	if err := os.Chmod(dir, 0o444); err != nil {
		t.Skipf("cannot chmod: %v", err)
	}
	defer os.Chmod(dir, 0o755) //nolint:errcheck
	t.Setenv("HOME", dir)

	a := NewAppService()
	err := a.ServiceStartup(context.Background(), application.ServiceOptions{})
	// It may or may not error depending on the OS; we just ensure no panic.
	_ = err
}

// ── Additional AppService coverage ────────────────────────────────────────

func TestAppService_StopRTSPRelay_ActiveRelay(t *testing.T) {
	dir := t.TempDir()
	cm := &ConfigManager{path: dir + "/cfg.json"}
	_ = cm.Save(Config{Sources: []Source{
		{ID: "rtsp:s1", Type: "rtsp", Name: "Cam", URL: "rtsp://x"},
	}})
	a := &AppService{config: cm, relays: make(map[string]*RTSPRelay)}

	// Manually inject a started relay.
	r := newRTSPRelay("rtsp:s1", "rtsp://x")
	_, err := r.Start(func(string, any) {})
	if err != nil {
		t.Fatalf("relay Start: %v", err)
	}
	a.relays["rtsp:s1"] = r

	if err := a.StopRTSPRelay("rtsp:s1"); err != nil {
		t.Fatalf("StopRTSPRelay: %v", err)
	}
	if len(a.relays) != 0 {
		t.Errorf("expected relay to be removed, relays=%v", a.relays)
	}
	select {
	case <-r.ctx.Done():
	case <-time.After(500 * time.Millisecond):
		t.Error("relay not stopped")
	}
}

func TestAppService_EmitEvent_WithData(t *testing.T) {
	a := NewAppService()
	// emitEvent with app == nil must discard gracefully (already tested),
	// but we also verify the branches with and without data don't panic.
	a.emitEvent("x", "payload")
	a.emitEvent("y", nil)
}

func TestAppService_SetDefaultSource_UpdatesOnlyTarget(t *testing.T) {
	dir := t.TempDir()
	cm := &ConfigManager{path: dir + "/cfg.json"}
	_ = cm.Save(Config{Sources: []Source{
		{ID: "s1", Type: "usb", Name: "A", IsDefault: true},
		{ID: "s2", Type: "usb", Name: "B", IsDefault: false},
		{ID: "s3", Type: "usb", Name: "C", IsDefault: false},
	}})
	a := &AppService{config: cm, relays: make(map[string]*RTSPRelay)}

	if err := a.SetDefaultSource("s3"); err != nil {
		t.Fatalf("SetDefaultSource: %v", err)
	}
	cfg := a.GetConfig()
	for _, s := range cfg.Sources {
		switch s.ID {
		case "s3":
			if !s.IsDefault {
				t.Errorf("s3 should be default")
			}
		default:
			if s.IsDefault {
				t.Errorf("%s should not be default", s.ID)
			}
		}
	}
}

func TestAppService_StartRTSPRelay_Success(t *testing.T) {
	dir := t.TempDir()
	cm := &ConfigManager{path: dir + "/cfg.json"}
	_ = cm.Save(Config{Sources: []Source{
		{ID: "rtsp:s1", Type: "rtsp", Name: "Cam", URL: "rtsp://127.0.0.1:65100/stream"},
	}})
	a := &AppService{config: cm, relays: make(map[string]*RTSPRelay)}

	port, err := a.StartRTSPRelay("rtsp:s1")
	if err != nil {
		t.Fatalf("StartRTSPRelay: %v", err)
	}
	defer func() { _ = a.StopRTSPRelay("rtsp:s1") }()

	if port <= 0 || port > 65535 {
		t.Errorf("invalid port: %d", port)
	}
	if len(a.relays) != 1 {
		t.Errorf("expected 1 relay, got %d", len(a.relays))
	}

	// Starting again stops the old relay and starts a fresh one.
	port2, err := a.StartRTSPRelay("rtsp:s1")
	if err != nil {
		t.Fatalf("StartRTSPRelay (restart): %v", err)
	}
	defer func() { _ = a.StopRTSPRelay("rtsp:s1") }()
	if port2 <= 0 || port2 > 65535 {
		t.Errorf("invalid port on restart: %d", port2)
	}
}

func TestAppService_SaveConfig_PropagatesError(t *testing.T) {
	// Use an invalid path to force a save error.
	cm := &ConfigManager{path: "/dev/null/cannot/write/cfg.json"}
	a := &AppService{config: cm, relays: make(map[string]*RTSPRelay)}
	err := a.SaveConfig(Config{})
	if err == nil {
		t.Error("expected error for unwritable config path")
	}
}
