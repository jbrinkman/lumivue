## Context

See `proposal.md` for motivation. The RTSP path already proves the architecture: a Go-side relay reads frames, decodes them with `astiav` (FFmpeg C bindings), and serves an MJPEG stream over a local HTTP server. The frontend renders that stream in an `<img>` element. The USB camera path currently attempts to use the browser MediaDevices API, which is unavailable in the Wails WKWebView, so it always fails.

## Goals / Non-Goals

**Goals:**
- Implement a Go-side USB camera capture relay that mirrors the RTSP relay's interface (start → port, stop).
- Reuse the existing `astiav`/FFmpeg build setup and the existing MJPEG serving code where it makes sense.
- Resolve a configured camera name to an AVFoundation device index at runtime.
- Update the frontend to request the Go relay and render MJPEG instead of using `getUserMedia`.
- Emit lifecycle/error events so the main and projection windows can show status.
- Keep the `usb` source type in config unchanged.

**Non-Goals:**
- Supporting operating systems other than macOS in this change (the `avfoundation` backend is macOS-specific).
- Replacing the RTSP relay or unifying it with the USB relay into a generic abstraction.
- Changing the camera configuration UI or persistence.
- Solving name-stability of USB cameras across reboots (handled by config/user).

## Decisions

### 1. Create a separate `USBCameraRelay` struct
A new struct similar to `RTSPRelay` will own the AVFoundation capture, HTTP server, and subscriber broadcast.

- **Why**: USB capture has different setup (device enumeration, AVFoundation options) and different error semantics than RTSP. A separate struct keeps the lifecycle explicit and avoids complicating the already-working RTSP relay.
- **Alternative considered**: Refactor both relays into a generic `MJPEGRelay`. Rejected because the capture sources are different enough that the shared abstraction would require heavy parameterization now; duplication is small and easier to unify later once patterns stabilize.
- **Shape**: `sourceID`, `cameraName`, `port`, `server`, `ctx/cancel`, `subscribers map[chan []byte]struct{}`. The subscriber/broadcast/serve logic is the same as `RTSPRelay`, so it can be copied verbatim or extracted into a small private helper if the duplication is bothersome during implementation.

### 2. Device resolution via a cgo helper on macOS
Camera names are currently obtained from `system_profiler` in `GetUSBCameras()`. AVFoundation device names may not match exactly, so a small cgo file (`usb_camera_darwin.go`) will call `avdevice_list_devices` on the `avfoundation` input format to get the list of `(index, name, uid)`. The configured name is matched against the AVFoundation name with a tolerant substring/fuzzy check; on failure the capture returns an error.

- **Why**: `astiav` v0.42.0 registers devices but does not expose the device-list API. Adding a thin cgo call is the smallest change that uses the same FFmpeg libraries already linked.
- **Alternative considered**: Shell out to `ffmpeg -list_devices`. Rejected because the `ffmpeg` binary is not guaranteed to be present on end-user machines, even though the shared libraries are.
- **Alternative considered**: Use `system_profiler` output order as the AVFoundation index. Rejected because there is no documented correlation between `system_profiler` order and AVFoundation device indices.
- **Testability**: The device-resolution function will take a lister interface so tests can inject a fake device list without needing real hardware.

### 3. Capture loop with `astiav`
The relay will open the `avfoundation` input format using `formatContext.OpenInput` with a dictionary containing `video_device_index`, `framerate`, and `video_size` (e.g. `640x480` as a safe default). It will read packets, decode to `astiav.Frame`, convert to RGBA with a `SoftwareScaleContext`, and encode to JPEG.

- **Why**: This matches the existing RTSP H.264 → RGBA → JPEG pipeline and reuses the same `astiav` primitives.
- **Optimization note**: If the camera exposes an MJPEG format, the packet is already a JPEG and could be passed through. The first implementation will decode/encode to keep the pipeline uniform; a future optimization can detect MJPEG packets and skip the re-encode.
- **Frame rate**: AVFoundation's default `ntsc` framerate is strange for USB cameras; requesting `30` and a modest size keeps CPU use low.

### 4. Use the existing `<img>` element in the frontend
The RTSP stream already renders in an `<img>` via `multipart/x-mixed-replace`. USB will use the same pattern. The frontend's `camera.js` will call `StartUSBCamera(sourceID)`, get a port, and set the relevant `<img>` element's `src` to `http://127.0.0.1:<port>/stream`.

- **Why**: The `<video>` element cannot display an MJPEG stream served this way. Reusing the `<img>` path means the USB and RTSP flows differ only in which binding is called.
- **UI impact**: `main.js` will activate USB sources by showing the USB `<img>` element (or the unified feed `<img>`) and hiding the `<video>` element. The current `<video id="usb-video">` element in `renderAppShell` will be changed to an `<img>` (or a second `<img>` will be used).

### 5. Add `StartUSBCamera` / `StopUSBCamera` Wails bindings
`AppService` will grow:
- `StartUSBCamera(sourceID string) (int, error)`
- `StopUSBCamera(sourceID string) error`
- a map `usbRelays map[string]*USBCameraRelay`

`ServiceShutdown` will already stop all relays; `StartUSBCamera` will stop any existing relay for the same source before starting a new one.

- **Why**: The frontend needs explicit control over capture start/stop, just like RTSP.

### 6. Error events
The USB relay will emit `usb:error` events with the source ID and a message. `main.js` and `projection.js` will listen for `usb:error` and show status or error state.

- **Why**: Keeps the UI informed of permission or disconnection failures without polling.

## Risks / Trade-offs

- [Risk] `avdevice_list_devices` requires cgo and `libavdevice` headers; the build already uses `astiav` cgo, but adding direct C calls raises the possibility of memory-management bugs.
  - **Mitigation**: Keep the cgo helper small and well-scoped; free the device list with `avdevice_free_list_devices`; test with a fake lister in Go.
- [Risk] `system_profiler` camera names and AVFoundation camera names may not match, causing device resolution to fail.
  - **Mitigation**: Use a tolerant matching algorithm (case-insensitive, whitespace-normalized, substring); if a camera consistently fails to match, the user can remove and re-add it after `GetUSBCameras` is switched to the same AVFoundation list (see Open Questions).
- [Risk] `astiav` capture uses CPU for decode/scale/encode, which can be significant at higher resolutions.
  - **Mitigation**: Default to a modest capture size (e.g. `640x480`) and 30 fps; expose `video_size` as an option only if needed later.
- [Risk] The `<video>` element is currently used for USB; switching to `<img>` may affect CSS or projection layout.
  - **Mitigation**: Both elements use the same `.video-el` class with `object-fit: contain`; the change is mostly the tag name and source assignment.
- [Risk] First camera access triggers the macOS permission dialog; `astiav` open may return a generic error if denied.
  - **Mitigation**: Map known FFmpeg/AVFoundation permission error strings to a user-facing `usb:error` event and rely on the existing `camera-error-display` change for UI text.

## Migration Plan

- No config migration: `usb` source records still contain `id`, `type`, `name`, and `isDefault`.
- The old `camera.js` `playUSBCamera` implementation using `getUserMedia` will be removed or replaced; rollback means restoring the old frontend `camera.js` and removing the Go USB relay.

## Open Questions

- Should `GetUSBCameras` be updated to use the AVFoundation device list instead of `system_profiler` so the names shown to the user exactly match the names used for capture? This is deferrable; the name-matching function in this change can be updated independently.
