## 1. Inline source removal

- [x] 1.1 Add an inline confirmation helper in `frontend/src/settings.js` that replaces a source row with a "Remove this source?" message plus **Confirm** and **Cancel** buttons, and call `SaveConfig` / `renderSettings` only when Confirm is clicked.
- [x] 1.2 Wire the helper to the USB Remove button (`handleUSBRemove`) and verify that confirming removes the camera and saves the config.
- [x] 1.3 Wire the helper to the RTSP Remove button (`handleRTSPRemove`) and verify that confirming removes the stream and saves the config.
- [x] 1.4 Verify that clicking Cancel returns the row to its normal state without calling `SaveConfig`.

## 2. Inline RTSP stream rename

- [x] 2.1 Replace `window.prompt()` in `handleRTSPEdit` with the same inline rename form already used for USB sources, updating the source name and saving the config.
- [x] 2.2 Verify that renaming an RTSP stream works and that Cancel restores the original row.

## 3. Styling and regression testing

- [x] 3.1 Add minimal CSS to `frontend/src/style.css` for the inline confirmation and rename states (e.g., danger color for Confirm, spacing for the button group) and confirm the panel renders correctly.
- [x] 3.2 Run `task check` (fmt, lint, tests) and `task build` to ensure no Go or frontend build regressions.
- [ ] 3.3 Manual verification: add a USB camera, click Remove, confirm, cancel, and remove a default source; confirm the source list is persisted after reopening the settings panel and that the app does not try to start a removed source.
