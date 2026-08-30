## Context

Lumivue is a Go + Wails v2 desktop application using a vanilla JS / Vite frontend (no UI framework). The app is a blank slate today — only the Wails boilerplate exists. The goal is to build a clean, distraction-free camera viewer that:

- Displays a live video feed from a selected USB camera or RTSP stream
- Lets the operator instantly switch between configured sources
- Can project the feed fullscreen, chrome-free, on a chosen monitor

The frontend runs inside a macOS WebView. There is no server — all state is managed in the Go backend and surfaced to the frontend via Wails-bound Go methods and runtime events.

## Goals / Non-Goals

**Goals:**
- Minimal, focused UI that gets out of the way of the video
- Sub-second source switching
- Reliable RTSP playback with auto-reconnect
- Fullscreen projection on a second monitor without any OS window chrome
- Persistent configuration (sources, default, last monitor) across restarts

**Non-Goals:**
- Recording or capturing video to disk
- Audio passthrough (video only)
- Network streaming / output (inbound RTSP only)
- Multi-source simultaneous display (mosaic/grid view) — single active source only
- Cross-platform support beyond macOS in this iteration

## Decisions

### Decision 1: UX Layout — Sidebar Switcher

**Options considered:**

```
Option A: Bottom Thumbnail Strip
+------------------------------------------+
|                                          |
|            ACTIVE VIDEO FEED             |
|                                          |
+------+------+------+------+---------+---+
| [C1] | [C2] | [C3] | [S1] |  [cfg] |[F]|
+------+------+------+------+---------+---+
  thumb  thumb  thumb  thumb  settings full

Option B: Collapsible Left Sidebar
+---------+--------------------------------+
| Camera1 |                               |
| Camera2 |     ACTIVE VIDEO FEED        |
|*Camera3*|                               |
| Stream1 |                               |
+---------+                               |
|[Config] |                               |
|[Project]+---------------------------------+

Option C: Overlay HUD (auto-hide)
+------------------------------------------+
|                                          |
|            ACTIVE VIDEO FEED             |
|  +----+----+----+----+  [cfg]  [full]   |
|  | C1 | C2 | C3 | S1 |                  |
|  +----+----+----+----+                  |
+------------------------------------------+
  Controls appear on hover, fade after 3s
```

**Chosen: Option B — Collapsible Left Sidebar**

Rationale:
- Gives each source a clear, readable name label rather than a small thumbnail
- Sidebar can be collapsed to a narrow icon-only strip, maximizing video area
- Persistent visibility avoids the discoverability problem of Option C's auto-hide
- Cleaner than the bottom strip (Option A) when source count grows beyond 4-5
- Aligns with the mental model: sources are a list, not a filmstrip

The sidebar shows:
- Source name (truncated with ellipsis if long)
- Source type icon (USB camera vs RTSP)
- Default star indicator (★) for the designated default
- Active source highlighted with an accent color background
- Collapse toggle button at top to reduce to icon-only mode

### Decision 2: Video Pipeline for USB Cameras — Browser MediaDevices API

**Options considered:**
- A) `getUserMedia` in the WebView frontend (browser API)
- B) AVFoundation via CGo in Go, push frames as base64 events to frontend
- C) FFmpeg subprocess, pipe MJPEG to local HTTP, render as `<img>` in frontend

**Chosen: Option A — `getUserMedia`**

Rationale:
- Zero Go-side complexity for USB cameras; the WebView has direct camera access
- Native hardware acceleration and codec support from the OS
- Lowest latency path
- Camera device IDs from `navigator.mediaDevices.enumerateDevices()` are used for both config enumeration and activation — no Go-side driver needed
- Trade-off: The Go backend cannot inspect or post-process USB camera frames; acceptable since this is a pure display app

Permission note: The Wails WebView requires `NSCameraUsageDescription` in `Info.plist` and the `com.apple.security.device.camera` entitlement in the macOS entitlements file.

### Decision 3: Video Pipeline for RTSP Streams — gortsplib + MJPEG relay

**Options considered:**
- A) gortsplib (pure Go RTSP library) decode → MJPEG HTTP relay → `<img>` in frontend
- B) FFmpeg subprocess pipe → MJPEG relay → `<img>` in frontend
- C) Go-side decode → base64 frame events via Wails runtime → canvas in frontend

**Chosen: Option A — gortsplib + MJPEG relay**

Rationale:
- Pure Go, no external binary dependency (no FFmpeg install requirement)
- gortsplib handles RTSP/RTP protocol; paired with a software H.264 decoder (e.g., `go-h264decoder` or image/jpeg extraction) it produces JPEG frames
- A minimal HTTP server on a random localhost port serves `multipart/x-mixed-replace` MJPEG; the frontend renders it as a plain `<img src="http://localhost:<port>/stream">` — the simplest possible frontend integration
- Auto-reconnect logic lives in Go, independent of the frontend
- Trade-off: MJPEG relay has higher bandwidth than H.264 but is acceptable for local network / localhost use; for truly bandwidth-constrained RTSP this can be revisited

The MJPEG relay port is assigned dynamically at startup and communicated to the frontend via a Wails bound method.

### Decision 4: Fullscreen Projection — Second Wails Window

**Options considered:**
- A) Second `wails.Window` opened on target screen, frameless + fullscreen
- B) CSS Fullscreen API (`element.requestFullscreen()`) — restricted to the current window's monitor
- C) Open a separate OS-level window via `runtime.WindowSetPosition` tricks

**Chosen: Option A — Second Wails Window**

Rationale:
- Wails v2 supports multiple windows with independent display/position control via `application.NewWebviewWindow()` and `runtime` screen APIs
- A second window can be positioned to the target screen and set to frameless + maximized, giving true chrome-free fullscreen
- The projection window runs the same frontend bundle but in a minimal "projection mode" (no sidebar, no controls, video only) controlled by a URL query param or a Wails window name check
- Trade-off: The projection window is a full WebView instance (memory cost); acceptable for a presentation tool on modern hardware

### Decision 5: Configuration Storage — JSON in UserConfigDir

Config stored as `~/Library/Application Support/lumivue/config.json` on macOS (via `os.UserConfigDir()`). Schema:

```json
{
  "sources": [
    { "id": "usb:0x...deviceId", "type": "usb", "name": "Logitech BRIO", "default": true },
    { "id": "rtsp://192.168.1.10/cam1", "type": "rtsp", "name": "Back Door Cam", "url": "rtsp://..." }
  ],
  "lastMonitorIndex": 1
}
```

USB device IDs use the browser's `MediaDeviceInfo.deviceId` (a persistent per-origin string on macOS). Config read/write handled in Go via a `ConfigManager` struct with file-lock safety.

### Decision 6: Frontend Stack — Vanilla JS + CSS Custom Properties

No framework added. The UI is simple enough (sidebar list + video element) that a framework would add more complexity than it removes. Vanilla JS with ES modules. CSS custom properties for theming (dark by default). No build-time dependencies beyond the existing Vite setup.

## Risks / Trade-offs

**RTSP codec support** → gortsplib + software decode covers H.264 (most common); H.265/HEVC streams will not be supported in this iteration. Mitigation: Document the H.264 limitation; add H.265 support as a follow-on change.

**USB device ID persistence across reconnects** → Browser `deviceId` for a given camera can change if the device is unplugged and replugged on a different USB port (macOS-specific behavior). Mitigation: Match by `label` (device name) as a fallback; surface a warning if a configured device is not found by ID or label.

**Camera permission on first launch** → macOS will prompt for camera access. If denied, USB cameras are unavailable. Mitigation: Show a clear in-app error with instructions to grant access in System Settings > Privacy > Camera.

**Second window memory overhead** → Each Wails WebView is ~100–150 MB. With one projection window this is ~300 MB total. Mitigation: Only create the projection window on demand; destroy it when projection ends.

**RTSP reconnect during source switch** → If a switch happens while a reconnect is in progress, the relay goroutine must be cleanly cancelled. Mitigation: Each relay runs with a `context.Context` tied to the source's lifetime; switching cancels the old context before starting the new relay.

## Open Questions

- **Sidebar collapse persistence**: Should the sidebar collapse state (expanded vs icon-only) persist across launches? (Assumption: yes, stored in config.)
- **RTSP authentication**: Should we support `rtsp://user:pass@host/path` URLs, or require the user to embed credentials in the URL? (Assumption: URL-embedded credentials only for now; a credential field can be added later.)
- **Source ordering**: Should the user be able to reorder sources in the sidebar? (Assumption: config-file order; drag-to-reorder is out of scope for this change.)
- **gortsplib H.264 decoder**: The exact Go H.264 decoder library (mediadevices codec vs manual RTP→JPEG extraction) needs a spike to confirm frame-rate and latency are acceptable before committing to the approach.
