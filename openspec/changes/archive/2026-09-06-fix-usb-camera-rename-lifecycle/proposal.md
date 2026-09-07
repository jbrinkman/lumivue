## Why

Switching to a previously renamed USB camera currently fails to start the stream and then crashes the application. Renaming only changes the visible label, but the backend tries to match that label against the system device list, so the device is not found. The crash is a separate `USBCameraRelay` lifecycle race: `Stop` frees decoder state while the `captureLoop` goroutine is still running, causing a nil pointer panic when the loop's defer runs. A similar crash is reproducible when projecting a USB camera to full screen, because that path starts a second relay before the first one has fully stopped. Both issues must be fixed for source selection and projection to work safely.

## What Changes

- Add a `DisplayName` field to the `Source` model so USB sources keep the original system device name for matching while allowing users to set a custom label.
- Update `frontend/src/settings.js` rename flow so that for USB sources `DisplayName` is edited, while `Name` (the system device name used to resolve the camera) and `ID` remain unchanged.
- Update `frontend/src/main.js`, `frontend/src/projection.js`, and `frontend/src/sidebar.js` (or any source-label rendering) to show `DisplayName` when present, falling back to `Name`.
- Update `AppService.StartUSBCamera` and the `USBCameraRelay` to resolve the camera using `Name` (the system device name) and send `DisplayName` to the frontend for status messages.
- Fix the `USBCameraRelay` start/stop lifecycle:
  - Ensure `Stop` waits for the `captureLoop` goroutine to finish before freeing `decoderCtx` and `swsCtx`.
  - Guard `captureLoop` defers with nil checks before calling `Free()`.
  - Make `Start` return an error and not leave a partially-initialized relay when `resolveUSBCamera` or the initial `openDevice` call fails.
  - Emit failures that occur after streaming begins as `usb:error` events so the frontend stops and removes the failed camera display and shows an error notification without closing the application.
  - Prevent `StartUSBCamera` from starting a new relay while an old one for the same source is still shutting down.
- Add tests for the rename resolution path and the relay stop/start lifecycle.

## Capabilities

### New Capabilities

- `settings/rename-source`: Renaming a configured source must only change the user-facing display label; it must not break the underlying identifier used to locate or start the source.
- `usb-camera/relay-lifecycle`: The USB camera relay must start and stop cleanly without races, nil pointer panics, or leaked goroutines when the user switches sources.

### Modified Capabilities

- None.

## Impact

- `Source` JSON schema gains an optional `displayName` field. Existing configs without the field continue to work.
- `frontend/src/settings.js`, `frontend/src/main.js`, `frontend/src/projection.js`, and `frontend/src/sidebar.js` need to render `DisplayName`.
- `usb_camera_relay.go` needs concurrency fixes and nil guards.
- `app.go` `StartUSBCamera` should pass the correct camera name to the relay.
- New or updated Go tests in `usb_camera_relay_test.go` and `usb_camera_test.go` / `app_test.go`.
