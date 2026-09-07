## 1. Project Setup & Boilerplate Replacement

- [ ] 1.1 Add `github.com/bluenviron/gortsplib/v4` to `go.mod` / `go.sum` for RTSP support
- [ ] 1.2 Add a Go H.264 → JPEG frame decoder dependency (spike: confirm `github.com/asticode/go-astiav` or equivalent provides acceptable frame-rate on macOS)
- [ ] 1.3 Remove the `Greet` boilerplate method from `app.go`
- [ ] 1.4 Replace `frontend/index.html` and `frontend/src/` boilerplate with the camera-viewer app shell (empty video area + sidebar structure, no logic yet)
- [ ] 1.5 Add `NSCameraUsageDescription` to `build/darwin/Info.plist`
- [ ] 1.6 Add `com.apple.security.device.camera` entitlement to the macOS entitlements file

## 2. Configuration Manager (Go)

- [ ] 2.1 Define `Source` struct (ID, type USB/RTSP, name, URL, isDefault) and `Config` struct (Sources slice, LastMonitorIndex)
- [ ] 2.2 Implement `ConfigManager` with `Load()` and `Save()` using `os.UserConfigDir()` path (`~/Library/Application Support/lumivue/config.json`)
- [ ] 2.3 Expose `GetConfig() Config` as a Wails-bound method
- [ ] 2.4 Expose `SaveConfig(config Config) error` as a Wails-bound method

## 3. USB Camera Enumeration (Frontend)

- [ ] 3.1 On settings screen open, call `navigator.mediaDevices.getUserMedia({video:true})` to trigger permission prompt, then stop the stream immediately
- [ ] 3.2 Call `navigator.mediaDevices.enumerateDevices()` and filter to `videoinput` devices; return the list to the settings UI
- [ ] 3.3 Render the enumerated device list in the settings UI with checkboxes; pre-check devices whose `deviceId` or `label` matches a configured USB source
- [ ] 3.4 On save, call `SaveConfig` with the updated USB source selections (include/exclude) and any name overrides

## 4. RTSP Stream Configuration (Frontend + Go)

- [ ] 4.1 Add an "Add Stream" form in settings UI (fields: display name, RTSP URL)
- [ ] 4.2 Validate URL client-side: must begin with `rtsp://` or `rtsps://`; show error inline on invalid input
- [ ] 4.3 Check for duplicate URLs before adding; show warning if duplicate detected
- [ ] 4.4 Render existing RTSP sources in settings with edit (name/URL) and delete controls
- [ ] 4.5 On save, call `SaveConfig` with updated RTSP source list

## 5. Default Source & Sidebar Switcher UI

- [ ] 5.1 Build the left sidebar component: renders a list of all configured sources with name, type icon, and default star (★)
- [ ] 5.2 Highlight the active source entry with an accent color background
- [ ] 5.3 Wire each source entry to an `activateSource(sourceId)` JS function that triggers the appropriate video pipeline
- [ ] 5.4 Add a collapse/expand toggle button to the sidebar; persist collapse state in localStorage
- [ ] 5.5 Add keyboard arrow-key navigation and Enter-to-activate within the sidebar
- [ ] 5.6 Expose `SetDefaultSource(sourceId string) error` as a Wails-bound Go method and wire a right-click or long-press context menu in the sidebar to set default

## 6. USB Camera Video Playback (Frontend)

- [ ] 6.1 Implement `activateUSBCamera(deviceId)`: call `getUserMedia({ video: { deviceId: { exact: deviceId } } })` and set the result as the `srcObject` of the main `<video>` element
- [ ] 6.2 Stop any previously active `MediaStream` before activating a new one
- [ ] 6.3 Handle `getUserMedia` permission denial: show error state with instructions to grant access in System Settings
- [ ] 6.4 Apply CSS `object-fit: contain` on the video element to maintain aspect ratio with letterbox/pillarbox behavior
- [ ] 6.5 Show empty-state placeholder (graphic + prompt) when no sources are configured

## 7. RTSP Relay (Go)

- [ ] 7.1 Implement `RTSPRelay` struct: open RTSP connection via gortsplib, decode H.264 RTP packets to JPEG frames
- [ ] 7.2 Serve decoded frames as `multipart/x-mixed-replace` MJPEG stream on a random localhost port
- [ ] 7.3 Implement 5-second connection timeout; on failure emit a Wails event `rtsp:error` with the source ID and error message
- [ ] 7.4 Implement auto-reconnect: on unexpected disconnect, wait 3 seconds and attempt one reconnect; emit `rtsp:reconnecting` and `rtsp:error` events accordingly
- [ ] 7.5 Each relay runs with a `context.Context`; cancelling the context stops the relay and closes the HTTP listener cleanly
- [ ] 7.6 Expose `StartRTSPRelay(sourceId string) (port int, err error)` as a Wails-bound method
- [ ] 7.7 Expose `StopRTSPRelay(sourceId string) error` as a Wails-bound method

## 8. RTSP Video Playback (Frontend)

- [ ] 8.1 Implement `activateRTSPStream(sourceId)`: call `StartRTSPRelay` to get the MJPEG port, then set the main `<img>` element's `src` to `http://localhost:<port>/stream`
- [ ] 8.2 On `rtsp:reconnecting` event, show a "Reconnecting…" overlay on the video area
- [ ] 8.3 On `rtsp:error` event, hide the reconnecting overlay and show an error state with source name and a "Retry" button that calls `StartRTSPRelay` again
- [ ] 8.4 Stop the previous relay (`StopRTSPRelay`) before starting a new one during source switch

## 9. Monitor Enumeration & Fullscreen Projection (Go + Frontend)

- [ ] 9.1 Expose `GetScreens() []Screen` as a Wails-bound method using `runtime.ScreenGetAll` to list connected monitors (index, name, width, height, isPrimary)
- [ ] 9.2 Expose `StartProjection(screenIndex int) error` as a Wails-bound method: create a frameless, fullscreen Wails secondary window positioned to the target screen
- [ ] 9.3 The projection window loads the same frontend bundle with a `?mode=projection` query param; the frontend hides the sidebar and renders only the active video feed
- [ ] 9.4 Expose `StopProjection() error` as a Wails-bound method: close the secondary window
- [ ] 9.5 Handle Escape key in the projection window: emit a Wails event that triggers `StopProjection` in Go
- [ ] 9.6 Handle external close of the projection window (OS-level): reset projection state in the main window UI
- [ ] 9.7 Save `lastMonitorIndex` to config when a projection session starts; pre-select it on next open

## 10. Projection Control UI (Frontend)

- [ ] 10.1 Add a "Project" button/icon to the main window UI (e.g., in the sidebar footer or top bar)
- [ ] 10.2 On click, fetch the screen list via `GetScreens`, show a small popover or modal listing available monitors
- [ ] 10.3 Pre-select the last-used monitor (from config); allow user to select any listed monitor
- [ ] 10.4 On confirm, call `StartProjection(screenIndex)` and change the Project button to a "Stop" state
- [ ] 10.5 On "Stop Projection" click (or on `projection:closed` event from Go), call `StopProjection` and reset button state

## 11. App Shell & Settings Navigation

- [ ] 11.1 Add a settings icon/button in the sidebar or top bar that toggles the settings panel (camera config and RTSP config)
- [ ] 11.2 Settings panel: USB camera section (enumeration + checkboxes) and RTSP section (stream list + Add Stream form) on the same screen
- [ ] 11.3 Add a "Save" button in settings that calls `SaveConfig` and returns to the main video view
- [ ] 11.4 On app launch: load config via `GetConfig`, activate the default source (USB or RTSP), and populate the sidebar

## 12. Validation & Polish

- [ ] 12.1 Test USB camera permission prompt and denial error state on macOS
- [ ] 12.2 Test RTSP relay with a real or simulated RTSP stream; verify reconnect behavior
- [ ] 12.3 Test source switching in all combinations (USB→USB, USB→RTSP, RTSP→USB, RTSP→RTSP)
- [ ] 12.4 Test fullscreen projection on a second monitor; verify no window chrome is visible
- [ ] 12.5 Test config persistence: quit and relaunch; verify sources, default, and last monitor are restored
- [ ] 12.6 Run `task check` (fmt + vet + tests) and fix any issues
- [ ] 12.7 Update `README.md` with build instructions, permission requirements, and RTSP H.264-only limitation
