## Why

The settings panel currently relies on `window.confirm()` to confirm removal of a configured source. In the Wails WKWebView context, `window.confirm()` does not block or return the user's choice reliably, so clicking **Remove** for a previously added camera (or RTSP stream) appears to do nothing. The same `window.prompt()` dependency makes RTSP edit unreliable. We need an inline confirmation flow that works in the embedded webview.

## What Changes

- Replace the `window.confirm()` calls in `handleUSBRemove` and `handleRTSPRemove` with an inline confirmation UI rendered inside the settings row.
- Replace the `window.prompt()` call in `handleRTSPEdit` with the existing inline rename pattern already used for USB sources, so all source edits are consistent.
- Update `frontend/src/settings.js` to keep each remove handler state-local (no global modal) and to call `SaveConfig` / `renderSettings` only after the user confirms.
- Add minimal CSS in `frontend/src/style.css` for the confirmation row state (e.g., danger text + Confirm/Cancel buttons).
- Verify that removing a source that is currently the `defaultSource` clears or updates the default flag, so the app does not reference a missing source on startup.

## Capabilities

### New Capabilities

- `settings/remove-source`: The settings panel must allow the user to remove any configured source (USB camera or RTSP stream) and persist the updated source list. Confirmation must be handled inline without relying on `window.confirm()`.

### Modified Capabilities

- None. This is a pure interaction fix; the external behavior of source removal is unchanged except that it now actually works.

## Impact

- Affected files: `frontend/src/settings.js`, `frontend/src/style.css`.
- Config persistence (`SaveConfig` / `GetConfig`) remains unchanged; only the UI trigger path changes.
- No Go backend changes required.
