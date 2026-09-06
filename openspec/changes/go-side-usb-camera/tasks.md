## 1. AVFoundation device resolution

- [x] 1.1 Add a `usb_camera_darwin.go` file with a small cgo helper that calls `avdevice_list_devices` for the `avfoundation` input format and returns `(index, name, uid)` tuples. Wrap the cgo call behind a Go `deviceLister` interface so tests can inject a fake list. Verify with `go test` and `go vet`; coverage for the resolution package should be ≥80%.
- [x] 1.2 Implement camera-name matching (case-insensitive, whitespace-normalized, substring) between the configured source name and the AVFoundation device list. Add unit tests for exact, substring, and no-match cases. Verify with `go test -cover`.

## 2. USB camera relay core

- [x] 2.1 Create `usb_camera_relay.go` with the `USBCameraRelay` struct and `Start`, `Stop`, `subscribe`, `broadcast`, and `serveMJPEG` methods, mirroring the existing `RTSPRelay` patterns. Write `usb_camera_relay_test.go` covering start/stop, valid port, multiple subscribers, slow-subscriber drop, and the MJPEG HTTP endpoint. Verify `go test -cover` shows ≥80% for the relay.
- [x] 2.2 Add a `decodeFrameToJPEG` helper that converts an `astiav.Frame` to JPEG bytes using the existing RGBA → JPEG pipeline, and a packet path that handles MJPEG passthrough when the codec is `MJPEG`. Add unit tests using synthetic frames (e.g. a solid-color `astiav.Frame` or the existing test H.264 path) to verify JPEG output starts with the SOI marker and error paths return errors, not panics. Verify `go test -cover`.

## 3. Capture loop and lifecycle

- [x] 3.1 Implement the USB capture loop that opens the `avfoundation` device by index, sets `framerate`/`video_size` options, reads packets, decodes frames, and broadcasts JPEGs to subscribers. Include the `usb:error` emit path on failures. Because real USB hardware is not available in CI, structure the loop so it can be tested with an injected `astiav.FormatContext` or file-based format, and add tests for the happy path (packets → broadcast), permission-denied error path, and device-disconnected error path. Verify `go test -cover`.
- [x] 3.2 Add `StartUSBCamera` and `StopUSBCamera` methods to `AppService` in `app.go`, plus a `usbRelays` map and lifecycle management. Write tests in `app_test.go` (or extend existing tests) for missing source, wrong source type, start/stop, and `ServiceShutdown` cleanup. Verify `go test -cover` for the affected package.

## 4. Frontend bindings and playback

- [x] 4.1 Add `StartUSBCamera` and `StopUSBCamera` wrappers in `frontend/src/bindings.js`. Verify by checking the generated Wails runtime bindings or by importing them in `camera.js` and running `npm run build` in `frontend/`.
- [x] 4.2 Replace the `playUSBCamera` implementation in `frontend/src/camera.js` with a call to `StartUSBCamera(sourceID)` that returns an MJPEG port, sets the `<img>` source to `http://127.0.0.1:<port>/stream`, and listens for `usb:error` events. Verify by building the frontend (`npm run build`) and confirming no MediaDevices references remain in the USB playback path.
- [x] 4.3 Update `frontend/src/main.js` and `frontend/src/projection.js` to activate USB sources through the new `camera.js` path, show the feed `<img>` element, and surface `usb:error` events in the status overlay. If the markup currently uses `<video>` for USB, change it to `<img>` with the same `.video-el` styling. Verify by running `task build` and manually switching to a USB source in both main and projection windows.

## 5. Final verification

- [x] 5.1 Run `task check` from the repo root. Confirm `go fmt`, `go vet`, and `go test ./...` pass with no warnings and that overall package coverage for new Go files stays at or above 80%.
- [x] 5.2 Run `task build` and confirm the application binary builds. Manually test the full flow: add a USB camera in settings, select it, verify video appears in the main window, start projection, and verify the feed appears in the projection window; then disconnect the camera and confirm a `usb:error` status is shown.
