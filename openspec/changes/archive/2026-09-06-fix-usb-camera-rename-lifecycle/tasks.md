## 1. Data model and backend rename resolution

- [x] 1.1 Add an optional `DisplayName` field to the `Source` struct in `config.go` and update `config_test.go` to verify JSON round-trip with the new field.
- [x] 1.2 Update `AppService.StartUSBCamera` to pass `src.Name` (system device name) to the relay and use `src.DisplayName || src.Name` in log/frontend-facing messages.
- [x] 1.3 Add a Go test that creates a renamed USB source (`DisplayName` set, `Name` unchanged) and verifies `StartUSBCamera` resolves using `Name` and not `DisplayName`.

## 2. USB relay lifecycle fixes

- [x] 2.1 Add a `sync.WaitGroup` to `USBCameraRelay`, increment it in `capture`, and decrement it in `captureLoop` so `Stop` can wait for the goroutine to exit.
- [x] 2.2 Update `USBCameraRelay.Stop()` to cancel the context, shut down the HTTP server, wait for the capture goroutine, and only then free `swsCtx`; remove `decoderCtx` freeing from `Stop`.
- [x] 2.3 Add nil checks before all `Free()` calls in `captureLoop` and `Stop` to prevent nil-pointer panics.
- [x] 2.4 Add a Go test that starts a relay whose capture operation blocks after the initial device open, calls `Stop` from another goroutine, releases the capture operation, and verifies no panic or leaked goroutine.
- [x] 2.5 Add a Go test that calls `StartUSBCamera` twice in rapid succession for the same source and verifies no panic, no leaked goroutines, and the final relay is the active one.
- [x] 2.6 Add a Go test that exercises `StartUSBCamera` with a non-existent camera name and verifies it returns an error without launching a capture goroutine.
- [x] 2.7 Make the initial camera open part of synchronous relay startup and add a Go test verifying an `openDevice` failure is returned from `StartUSBCamera` without leaving a capture goroutine or active relay.

## 3. Frontend rename and display-name wiring

- [x] 3.1 Update `frontend/src/settings.js` so that renaming a USB source writes `DisplayName` and preserves `Name` and `ID`; rename of an RTSP source may edit `DisplayName` or `Name` as long as `URL` and `ID` are unchanged.
- [x] 3.2 Update `frontend/src/sidebar.js`, `frontend/src/main.js`, and `frontend/src/projection.js` to render a source label as `DisplayName || Name`.
- [x] 3.3 Verify that `handleConfigChange` in `main.js` still stops playback when a renamed source's `ID` is removed, because `ID` remains stable.
- [x] 3.4 Verify startup errors and runtime `usb:error` events both stop and remove the failed USB camera display and show an error notification without closing the application.

## 4. Integration and regression testing

- [x] 4.1 Run `task check` (fmt, lint, tests) and `task build` to ensure no Go or frontend build regressions.
- [x] 4.2 Manual verification: add a USB camera, rename it to a custom label, select it, confirm it starts and shows the custom label; then switch to another source and confirm no crash.
- [x] 4.3 Manual verification: with a USB camera already playing, trigger full-screen projection and confirm the app does not panic and the stream continues.
- [x] 4.4 Manual verification: remove a renamed USB source while it is playing and confirm the app does not panic and the configuration is saved.
