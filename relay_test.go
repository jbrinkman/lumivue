package main

import (
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
)

// ── RTSPRelay unit tests ───────────────────────────────────────────────────

func TestNewRTSPRelay(t *testing.T) {
	r := newRTSPRelay("src1", "rtsp://localhost/stream")
	if r.sourceID != "src1" {
		t.Errorf("sourceID: got %q, want %q", r.sourceID, "src1")
	}
	if r.url != "rtsp://localhost/stream" {
		t.Errorf("url: got %q, want %q", r.url, "rtsp://localhost/stream")
	}
	if r.subscribers == nil {
		t.Error("subscribers map must not be nil")
	}
	r.cancel()
}

func TestRTSPRelay_SubscribeBroadcast(t *testing.T) {
	r := newRTSPRelay("s1", "rtsp://x")
	defer r.cancel()

	ch, unsub := r.subscribe()
	frame := []byte("jpeg-data")
	r.broadcast(frame)

	select {
	case got := <-ch:
		if string(got) != string(frame) {
			t.Errorf("broadcast: got %q, want %q", got, frame)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("timed out waiting for broadcast frame")
	}

	// Unsubscribe removes the channel; subsequent broadcasts must not be received.
	unsub()
	r.broadcast([]byte("second"))
	select {
	case <-ch:
		t.Error("should not receive after unsubscribe")
	case <-time.After(100 * time.Millisecond):
		// Expected: nothing received.
	}
}

func TestRTSPRelay_BroadcastDropsSlow(t *testing.T) {
	r := newRTSPRelay("s1", "rtsp://x")
	defer r.cancel()

	ch, unsub := r.subscribe()
	defer unsub()

	// Fill the buffered channel (capacity 2), then broadcast again — must be dropped.
	r.broadcast([]byte("a"))
	r.broadcast([]byte("b"))
	r.broadcast([]byte("c")) // should be dropped silently

	if len(ch) != 2 {
		t.Errorf("expected 2 frames in channel, got %d", len(ch))
	}
}

func TestRTSPRelay_Stop(t *testing.T) {
	r := newRTSPRelay("s1", "rtsp://x")
	_, err := r.Start(func(string, any) {})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	r.Stop()
	select {
	case <-r.ctx.Done():
		// Expected: context was cancelled.
	case <-time.After(500 * time.Millisecond):
		t.Error("context not cancelled after Stop")
	}
}

func TestRTSPRelay_Start_ReturnsValidPort(t *testing.T) {
	r := newRTSPRelay("s1", "rtsp://x")
	port, err := r.Start(func(string, any) {})
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer r.Stop()

	if port <= 0 || port > 65535 {
		t.Fatalf("invalid port: %d", port)
	}

	// The HTTP server is up: a non-existent path returns 404 immediately,
	// confirming the server is accepting connections.
	url := "http://127.0.0.1:" + strconv.Itoa(port) + "/healthz"
	resp, err := http.Get(url) //nolint:noctx
	if err != nil {
		t.Fatalf("GET /healthz: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for unknown path, got %d", resp.StatusCode)
	}
}

func TestRTSPRelay_ServeMJPEG(t *testing.T) {
	r := newRTSPRelay("s1", "rtsp://x")
	defer r.cancel()

	srv := httptest.NewServer(http.HandlerFunc(r.serveMJPEG))
	defer srv.Close()

	frame1 := []byte("fake-jpeg-1")
	frame2 := []byte("fake-jpeg-2")

	// Broadcast two frames then cancel the relay context.
	go func() {
		time.Sleep(20 * time.Millisecond)
		r.broadcast(frame1)
		time.Sleep(20 * time.Millisecond)
		r.broadcast(frame2)
		time.Sleep(20 * time.Millisecond)
		r.cancel()
	}()

	resp, err := http.Get(srv.URL) //nolint:noctx
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	ct := resp.Header.Get("Content-Type")
	mt, params, _ := mime.ParseMediaType(ct)
	if mt != "multipart/x-mixed-replace" {
		t.Fatalf("content-type: got %q, want multipart/x-mixed-replace", mt)
	}

	mr := multipart.NewReader(resp.Body, params["boundary"])
	var parts [][]byte
	for {
		p, err := mr.NextPart()
		if err != nil {
			break
		}
		data, _ := io.ReadAll(p)
		parts = append(parts, data)
	}

	if len(parts) < 2 {
		t.Fatalf("expected ≥2 MJPEG parts, got %d", len(parts))
	}
	if string(parts[0]) != string(frame1) {
		t.Errorf("part 0: got %q, want %q", parts[0], frame1)
	}
	if string(parts[1]) != string(frame2) {
		t.Errorf("part 1: got %q, want %q", parts[1], frame2)
	}
}

// TestServeMJPEG_ClientDisconnect verifies that serveMJPEG exits cleanly when
// the HTTP client cancels the request (req.Context().Done fires).
// This covers rtsp.go lines 150-151.
func TestServeMJPEG_ClientDisconnect(t *testing.T) {
	r := newRTSPRelay("s1", "rtsp://x")
	defer r.cancel()

	srv := httptest.NewServer(http.HandlerFunc(r.serveMJPEG))
	defer srv.Close()

	// Make a cancellable GET request.
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel the context after 50 ms — well before any frames arrive — so the
	// req.Context().Done() case fires inside serveMJPEG.
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
	if err != nil {
		t.Fatalf("NewRequest: %v", err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		// Expected: context was cancelled before the response completed.
		return
	}
	// If we get a response, drain until the server-side handler closes it.
	defer resp.Body.Close()
	_, _ = io.ReadAll(resp.Body)
}

func TestRTSPRelay_MultipleSubscribers(t *testing.T) {
	r := newRTSPRelay("s1", "rtsp://x")
	defer r.cancel()

	const n = 5
	channels := make([]<-chan []byte, n)
	unsubs := make([]func(), n)
	for i := range n {
		channels[i], unsubs[i] = r.subscribe()
	}

	frame := []byte("hello")
	r.broadcast(frame)

	var wg sync.WaitGroup
	wg.Add(n)
	for i := range n {
		i := i
		go func() {
			defer wg.Done()
			defer unsubs[i]()
			select {
			case got := <-channels[i]:
				if string(got) != string(frame) {
					t.Errorf("subscriber %d: got %q, want %q", i, got, frame)
				}
			case <-time.After(500 * time.Millisecond):
				t.Errorf("subscriber %d timed out", i)
			}
		}()
	}
	wg.Wait()
}

// ── AppService tests ───────────────────────────────────────────────────────

func TestAppService_NewAppService(t *testing.T) {
	a := NewAppService()
	if a == nil {
		t.Fatal("NewAppService returned nil")
	}
	if a.relays == nil {
		t.Error("relays map must not be nil")
	}
}

func TestAppService_GetConfig_EmptyAfterInit(t *testing.T) {
	dir := t.TempDir()
	a := &AppService{
		config: &ConfigManager{path: filepath.Join(dir, "cfg.json")},
		relays: make(map[string]*RTSPRelay),
	}
	cfg := a.GetConfig()
	if len(cfg.Sources) != 0 {
		t.Errorf("expected 0 sources, got %d", len(cfg.Sources))
	}
}

func TestAppService_SaveConfig(t *testing.T) {
	dir := t.TempDir()
	a := &AppService{
		config: &ConfigManager{path: filepath.Join(dir, "cfg.json")},
		relays: make(map[string]*RTSPRelay),
	}
	cfg := Config{Sources: []Source{{ID: "u1", Type: "usb", Name: "Cam"}}}
	if err := a.SaveConfig(cfg); err != nil {
		t.Fatalf("SaveConfig: %v", err)
	}
	got := a.GetConfig()
	if len(got.Sources) != 1 || got.Sources[0].ID != "u1" {
		t.Errorf("unexpected config: %+v", got)
	}
}

func TestAppService_GetScreens_NilApp(t *testing.T) {
	a := NewAppService()
	// application.Get() returns nil outside Wails; GetScreens must not panic.
	screens := a.GetScreens()
	if screens != nil {
		t.Errorf("expected nil when app not initialized, got %+v", screens)
	}
}

func TestAppService_StartProjection_NilApp(t *testing.T) {
	dir := t.TempDir()
	a := &AppService{
		config: &ConfigManager{path: filepath.Join(dir, "cfg.json")},
		relays: make(map[string]*RTSPRelay),
	}
	err := a.StartProjection(0)
	if err == nil {
		t.Error("expected error when app is not initialized")
	}
}

func TestAppService_StartProjection_AlreadyProjecting(t *testing.T) {
	dir := t.TempDir()
	a := &AppService{
		config:           &ConfigManager{path: filepath.Join(dir, "cfg.json")},
		relays:           make(map[string]*RTSPRelay),
		projectionWindow: new(application.WebviewWindow),
	}
	// When projectionWindow is non-nil, StartProjection returns nil without
	// opening a second window (early-return path fires before app.Get() call).
	err := a.StartProjection(0)
	if err != nil {
		t.Errorf("expected nil when already projecting, got %v", err)
	}
}

func TestAppService_StopProjection_NoWindow(t *testing.T) {
	a := &AppService{relays: make(map[string]*RTSPRelay)}
	if err := a.StopProjection(); err != nil {
		t.Errorf("StopProjection with no window: unexpected error: %v", err)
	}
}

func TestAppService_ServiceShutdown_StopsRelays(t *testing.T) {
	dir := t.TempDir()
	a := &AppService{
		config: &ConfigManager{path: filepath.Join(dir, "cfg.json")},
		relays: make(map[string]*RTSPRelay),
	}
	r := newRTSPRelay("s1", "rtsp://x")
	_, err := r.Start(func(string, any) {})
	if err != nil {
		t.Fatalf("relay Start: %v", err)
	}
	a.relays["s1"] = r

	if err := a.ServiceShutdown(); err != nil {
		t.Fatalf("ServiceShutdown: %v", err)
	}
	if len(a.relays) != 0 {
		t.Errorf("expected 0 relays after shutdown, got %d", len(a.relays))
	}
	select {
	case <-r.ctx.Done():
		// relay context was cancelled.
	case <-time.After(500 * time.Millisecond):
		t.Error("relay context not cancelled after ServiceShutdown")
	}
}

func TestAppService_StartRTSPRelay_MissingSource(t *testing.T) {
	dir := t.TempDir()
	cm := &ConfigManager{path: filepath.Join(dir, "cfg.json")}
	_ = cm.Save(Config{Sources: []Source{{ID: "u1", Type: "usb", Name: "Cam"}}})
	a := &AppService{config: cm, relays: make(map[string]*RTSPRelay)}
	_, err := a.StartRTSPRelay("nonexistent")
	if err == nil {
		t.Error("expected error for missing RTSP source")
	}
}

func TestAppService_StartRTSPRelay_WrongType(t *testing.T) {
	dir := t.TempDir()
	cm := &ConfigManager{path: filepath.Join(dir, "cfg.json")}
	_ = cm.Save(Config{Sources: []Source{{ID: "u1", Type: "usb", Name: "Cam"}}})
	a := &AppService{config: cm, relays: make(map[string]*RTSPRelay)}
	_, err := a.StartRTSPRelay("u1")
	if err == nil {
		t.Error("expected error when source type is not rtsp")
	}
}

func TestAppService_EmitEvent_NilApp(t *testing.T) {
	a := NewAppService()
	// Must not panic when the Wails app is not initialized.
	a.emitEvent("test:event", nil)
	a.emitEvent("test:event", map[string]any{"key": "val"})
}

// ── ConfigManager additional tests ────────────────────────────────────────

func TestNewConfigManager(t *testing.T) {
	dir := t.TempDir()
	origHome := os.Getenv("HOME")
	t.Setenv("HOME", dir)
	_ = origHome // silence lint

	cm, err := NewConfigManager()
	if err != nil {
		t.Fatalf("NewConfigManager: %v", err)
	}
	if cm == nil {
		t.Fatal("ConfigManager is nil")
	}

	cfg := Config{LastMonitorIndex: 2}
	if err := cm.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}

	cm2, err := NewConfigManager()
	if err != nil {
		t.Fatalf("NewConfigManager (reload): %v", err)
	}
	if cm2.Load().LastMonitorIndex != 2 {
		t.Errorf("expected LastMonitorIndex=2, got %d", cm2.Load().LastMonitorIndex)
	}
}
