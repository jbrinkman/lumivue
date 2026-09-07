## 1. Data model and backend rename resolution

- [ ] 1.1 Add an optional `DisplayName` field to the `Source` struct in `config.go` and update `config_test.go` to verify JSON round-trip with the new field.
- [ ] 1.2 Update `AppService.StartUSBCamera` to pass `src.Name` (system device name) to the relay and use `src.DisplayName || src.Name` in log/frontend-facing messages.
- [ ] 1.3 Add a Go test that creates a renamed USB source (`DisplayName` set, `Name` unchanged) and verifies `StartUSBCamera` resolves using `Name` and not `DisplayName`.

## 2. USB relay lifecycle fixes

- [ ] 2.1 Add a `sync.WaitGroup` to `USBCameraRelay`, increment it in `capture`, and decrement it in `captureLoop` so `Stop` can wait for the goroutine to exit.
- [ ] 2.2 Update `USBCameraRelay.Stop()` to cancel the context, shut down the HTTP server, wait for the capture goroutine, and only then free `swsCtx`; remove `decoderCtx` freeing from `Stop`.
- [ ] 2.3 Add nil checks before all `Free()` calls in `captureLoop` and `Stop` to prevent nil-pointer panics.
- [ ] 2.4 Add a Go test that starts a relay with a fake `openDevice` that blocks, calls `Stop` from another goroutine, and verifies no panic or leaked goroutine.
- [ ] 2.5 Add a Go test that calls `StartUSBCamera` twice in rapid succession for the same source and verifies no panic, no leaked goroutines, and the final relay is the active one.
- [ ] 2.6 Add a Go test that exercises `StartUSBCamera` with a non-existent camera name and verifies it returns an error without launching a capture goroutine.

## 3. Frontend rename and display-name wiring

- [ ] 3.1 Update `frontend/src/settings.js` so that renaming a USB source writes `DisplayName` and preserves `Name` and `ID`; rename of an RTSP source may edit `DisplayName` or `Name` as long as `URL` and `ID` are unchanged.
- [ ] 3.2 Update `frontend/src/sidebar.js`, `frontend/src/main.js`, and `frontend/src/projection.js` to render a source label as `DisplayName || Name`.
- [ ] 3.3 Verify that `handleConfigChange` in `main.js` still stops playback when a renamed source's `ID` is removed, because `ID` remains stable.

## 4. Integration and regression testing

- [ ] 4.1 Run `task check` (fmt, lint, tests) and `task build` to ensure no Go or frontend build regressions.
- [ ] 4.2 Manual verification: add a USB camera, rename it to a custom label, select it, confirm it starts and shows the custom label; then switch to another source and confirm no crash.
- [ ] 4.3 Manual verification: with a USB camera already playing, trigger full-screen projection and confirm the app does not panic and the stream continues.
- [ ] 4.4 Manual verification: remove a renamed USB source while it is playing and confirm the app does not panic and the configuration is saved.
