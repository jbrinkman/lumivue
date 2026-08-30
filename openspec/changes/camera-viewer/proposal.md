## Why

Lumivue is a purpose-built camera viewer: a clean, distraction-free Wails desktop app that lets a presenter or operator instantly switch between any configured USB camera or RTSP stream and project the live feed fullscreen on a chosen monitor. The app ships as a blank Wails slate today and needs its core feature set implemented.

## What Changes

- Add camera configuration system: enumerate available USB cameras, let the user select a named subset (including a default), and persist RTSP stream definitions (name + URL)
- Add live video display in the Wails window using the browser MediaDevices API for USB cameras and a Go-side RTSP decoder with local MJPEG/HLS relay for RTSP streams
- Add a camera-switcher UI: shows the active feed prominently with a quick-switch control (thumbnail strip or sidebar list) for all other configured sources
- Add fullscreen projection: open a borderless, chrome-free window on any connected monitor showing the current feed
- Add monitor selection: enumerate connected displays so the user can choose which screen receives the fullscreen projection
- Replace the Wails vanilla boilerplate (Greet handler, Hello World frontend) with the camera-viewer application shell

## Capabilities

### New Capabilities

- `camera-config`: Define and persist the set of cameras and RTSP streams the user wants to use, including designating a default source
- `video-display`: Render a live camera or RTSP stream inside the Wails window with low-latency playback
- `camera-switcher`: UI control that shows all configured sources and allows instant switching; highlights the active source
- `fullscreen-projection`: Project the active feed as a borderless fullscreen window on a user-selected monitor

### Modified Capabilities

<!-- No existing specs — this is a greenfield application -->

## Impact

- **Go backend** (`app.go`): new structs and exported methods for camera enumeration, config read/write, RTSP relay, monitor enumeration, and fullscreen window management
- **Frontend** (`frontend/src/`): replace boilerplate with camera-viewer app shell; add switcher UI and fullscreen trigger
- **Dependencies**: add Go library for RTSP decoding/relay (gortsplib or ffmpeg subprocess); Wails runtime already covers multi-window and screen APIs
- **Config file**: new JSON/YAML config stored in the OS user-config directory (via `os.UserConfigDir`) for camera and stream definitions
- **Permissions**: macOS camera entitlement required in `build/darwin/Info.plist`
