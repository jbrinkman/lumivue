## Why

The frontend `camera.js` relies on `navigator.mediaDevices.getUserMedia` inside the Wails WKWebView, but `navigator.mediaDevices` is undefined there, so USB camera playback fails before it starts. The existing RTSP path already uses a Go-side FFmpeg/astiav decoder that serves MJPEG to the frontend. Moving USB camera capture to the Go side as well lets us reuse that proven pipeline, avoid WKWebView media API limitations, and keep the UI simple.

## What Changes

- Add a Go-side USB camera capture module that opens an AVFoundation device, decodes frames, and serves them as an MJPEG stream over a local HTTP server, mirroring the existing RTSP relay architecture.
- Add Wails bindings `StartUSBCamera(sourceID)` and `StopUSBCamera(sourceID)` so the frontend can start and stop the capture for a configured USB source.
- Update the frontend to request the Go-side USB stream instead of calling `getUserMedia`/`enumerateDevices`.
- Resolve a configured camera name to an AVFoundation device index at runtime.
- Stop the old browser-based USB playback path (`camera.js`) for configured sources; it is replaced by the Go relay.
- Emit lifecycle/error events for the USB camera capture so the main and projection windows can show status messages.

## Capabilities

### New Capabilities
- `usb-camera/go-capture`: Defines how the Go backend captures from a USB camera and serves it as an MJPEG stream to the frontend.

### Modified Capabilities
- `video-display`: The requirement to render the active USB camera feed via the browser MediaDevices API changes to rendering the feed via the Go-side MJPEG relay. The observable behavior (live video in the main window) stays the same, but the source mechanism and the failure modes change.

## Impact

- `app.go`: add `StartUSBCamera` and `StopUSBCamera` methods; manage `USBCameraRelay` instances alongside `RTSPRelay` instances.
- New Go files for the USB camera relay, device resolution, and frame decoding.
- `frontend/src/camera.js`: replace `getUserMedia` logic with bindings to the Go relay.
- `frontend/src/main.js` and `frontend/src/projection.js`: activate USB sources through the new `StartUSBCamera` binding and render the returned MJPEG URL in a `<img>` or `<video>` element.
- `frontend/src/bindings.js`: add `StartUSBCamera` and `StopUSBCamera` wrappers.
- Build / packaging: no new Go dependencies beyond the existing `astiav` FFmpeg bindings; the build already links FFmpeg through Homebrew and sets the correct macOS deployment target in `Taskfile.yaml`.
- macOS camera entitlement is already present in `build/darwin/entitlements.plist` and `Info.plist`.
