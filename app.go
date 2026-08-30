package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"sync"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// Screen is the simplified monitor description returned to the frontend.
type Screen struct {
	Index     int    `json:"index"`
	Name      string `json:"name"`
	Width     int    `json:"width"`
	Height    int    `json:"height"`
	IsPrimary bool   `json:"isPrimary"`
}

// AppService implements all backend functionality exposed to the Wails frontend.
type AppService struct {
	mu               sync.Mutex
	config           *ConfigManager
	relays           map[string]*RTSPRelay
	projectionWindow *application.WebviewWindow
}

// NewAppService creates an uninitialised AppService. Initialisation occurs in
// ServiceStartup when the Wails application is ready.
func NewAppService() *AppService {
	return &AppService{
		relays: make(map[string]*RTSPRelay),
	}
}

// ServiceStartup is called by Wails v3 when the application starts.
func (a *AppService) ServiceStartup(_ context.Context, _ application.ServiceOptions) error {
	cm, err := NewConfigManager()
	if err != nil {
		return fmt.Errorf("initialize config manager: %w", err)
	}
	a.config = cm
	return nil
}

// ServiceShutdown is called by Wails v3 when the application shuts down.
func (a *AppService) ServiceShutdown() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	for id := range a.relays {
		a.relays[id].Stop()
	}
	a.relays = make(map[string]*RTSPRelay)
	return nil
}

// ── Configuration ──────────────────────────────────────────────────────────

// GetConfig returns the current application configuration.
func (a *AppService) GetConfig() Config {
	return a.config.Load()
}

// SaveConfig persists the provided configuration to disk.
func (a *AppService) SaveConfig(cfg Config) error {
	if err := a.config.Save(cfg); err != nil {
		return fmt.Errorf("save config: %w", err)
	}
	return nil
}

// SetDefaultSource marks the source with the given ID as the default and
// clears the default flag from all other sources.
func (a *AppService) SetDefaultSource(sourceID string) error {
	cfg := a.config.Load()
	found := false
	for i := range cfg.Sources {
		if cfg.Sources[i].ID == sourceID {
			cfg.Sources[i].IsDefault = true
			found = true
		} else {
			cfg.Sources[i].IsDefault = false
		}
	}
	if !found {
		return fmt.Errorf("source %q not found", sourceID)
	}
	if err := a.config.Save(cfg); err != nil {
		return fmt.Errorf("save config after set default: %w", err)
	}
	return nil
}

// ── RTSP Relay ─────────────────────────────────────────────────────────────

// StartRTSPRelay starts an RTSP→MJPEG relay for the source with the given ID.
// Returns the localhost port on which the MJPEG stream is available.
func (a *AppService) StartRTSPRelay(sourceID string) (int, error) {
	cfg := a.config.Load()
	var src *Source
	for i := range cfg.Sources {
		if cfg.Sources[i].ID == sourceID && cfg.Sources[i].Type == "rtsp" {
			src = &cfg.Sources[i]
			break
		}
	}
	if src == nil {
		return 0, fmt.Errorf("RTSP source %q not found", sourceID)
	}

	a.mu.Lock()
	// Stop any existing relay for this source.
	if existing, ok := a.relays[sourceID]; ok {
		existing.Stop()
		delete(a.relays, sourceID)
	}
	relay := newRTSPRelay(sourceID, src.URL)
	a.relays[sourceID] = relay
	a.mu.Unlock()

	port, err := relay.Start(a.emitEvent)
	if err != nil {
		a.mu.Lock()
		delete(a.relays, sourceID)
		a.mu.Unlock()
		return 0, fmt.Errorf("start relay for %q: %w", sourceID, err)
	}
	return port, nil
}

// StopRTSPRelay stops the RTSP relay for the given source ID.
func (a *AppService) StopRTSPRelay(sourceID string) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	relay, ok := a.relays[sourceID]
	if !ok {
		return nil // already stopped; not an error
	}
	relay.Stop()
	delete(a.relays, sourceID)
	return nil
}

// ── USB Cameras ────────────────────────────────────────────────────────────

// spCameraData mirrors the JSON structure returned by
// `system_profiler SPCameraDataType -json`.
type spCameraData struct {
	SPCameraDataType []struct {
		Name string `json:"_name"`
	} `json:"SPCameraDataType"`
}

// GetUSBCameras returns the names of all cameras visible to macOS using
// system_profiler.  This does not require camera permission from the OS,
// so it works even before the user has granted access in System Settings.
func (a *AppService) GetUSBCameras() ([]string, error) {
	out, err := exec.Command("system_profiler", "SPCameraDataType", "-json").Output()
	if err != nil {
		return nil, fmt.Errorf("list cameras: %w", err)
	}
	var data spCameraData
	if err := json.Unmarshal(out, &data); err != nil {
		return nil, fmt.Errorf("parse camera list: %w", err)
	}
	names := make([]string, 0, len(data.SPCameraDataType))
	for _, c := range data.SPCameraDataType {
		if c.Name != "" {
			names = append(names, c.Name)
		}
	}
	return names, nil
}

// ── Monitor & Projection ───────────────────────────────────────────────────

// GetScreens returns the list of connected monitors.
func (a *AppService) GetScreens() []Screen {
	app := application.Get()
	if app == nil {
		return nil
	}
	screens := app.Screen.GetAll()
	result := make([]Screen, len(screens))
	for i, s := range screens {
		result[i] = Screen{
			Index:     i,
			Name:      s.Name,
			Width:     s.Size.Width,
			Height:    s.Size.Height,
			IsPrimary: s.IsPrimary,
		}
	}
	return result
}

// StartProjection opens a frameless fullscreen projection window on the
// monitor at the given index and saves lastMonitorIndex to config.
func (a *AppService) StartProjection(screenIndex int) error {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.projectionWindow != nil {
		return nil // already projecting
	}

	app := application.Get()
	if app == nil {
		return fmt.Errorf("application not initialized")
	}
	screen := app.Screen.GetByIndex(screenIndex)
	if screen == nil {
		return fmt.Errorf("screen index %d not found", screenIndex)
	}

	win := app.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:     "Lumivue Projection",
		Name:      "projection",
		Frameless: true,
		URL:       "/?mode=projection",
		Width:     screen.Size.Width,
		Height:    screen.Size.Height,
		Mac: application.MacWindow{
			Backdrop: application.MacBackdropNormal,
		},
	})
	win.SetScreen(screen)
	win.Fullscreen()
	a.projectionWindow = win

	// Listen for the OS closing the projection window externally.
	win.OnWindowEvent(events.Common.WindowClosing, func(_ *application.WindowEvent) {
		a.mu.Lock()
		if a.projectionWindow == win {
			a.projectionWindow = nil
		}
		a.mu.Unlock()
		a.emitEvent("projection:closed", nil)
	})

	// Save lastMonitorIndex to config.
	cfg := a.config.Load()
	cfg.LastMonitorIndex = screenIndex
	_ = a.config.Save(cfg)

	return nil
}

// StopProjection closes the projection window if open.
func (a *AppService) StopProjection() error {
	a.mu.Lock()
	win := a.projectionWindow
	a.projectionWindow = nil
	a.mu.Unlock()

	if win != nil {
		win.Close()
	}
	return nil
}

// ── Helpers ────────────────────────────────────────────────────────────────

// emitEvent sends a named event (with optional data) to all Wails windows.
func (a *AppService) emitEvent(name string, data any) {
	app := application.Get()
	if app == nil {
		return
	}
	if data != nil {
		app.Event.Emit(name, data)
	} else {
		app.Event.Emit(name)
	}
}
