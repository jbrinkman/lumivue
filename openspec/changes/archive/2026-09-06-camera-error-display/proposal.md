## Why

The USB camera playback path in `camera.js` throws a generic `Camera access is not available in this context.` error when `navigator.mediaDevices` is undefined inside the Wails WKWebView. That message is captured in `main.js` and `projection.js` and shown as a plain, technical string, which does not explain to the user why the camera cannot start or what they can do next. A clearer, more actionable error state will make the failure obvious and prepare the user for the eventual Go-side camera implementation.

## What Changes

- Update the camera error UI in the main window so it shows a friendly, contextual message instead of a raw JavaScript error string.
- Distinguish between the "camera access unavailable in this webview" case, the "permission denied" case, and the "camera not found" case.
- Show the same contextual error in the projection window when a USB camera fails to play there.
- Add a small hint or call-to-action (e.g., "Camera playback requires a native implementation and is not yet supported in this build") for the webview-unavailable case.
- Keep the existing error-state styling and `showError` / `showStatus` helpers; extend them rather than replacing the UI shell.
- No changes to Go bindings, camera enumeration, or configuration persistence.

## Capabilities

### New Capabilities
- `camera/error-display`: Defines how the application surfaces USB camera playback errors to the user in both the main and projection windows, including distinct messages for unavailable APIs, denied permissions, and missing devices.

### Modified Capabilities
- None.

## Impact

- `frontend/src/camera.js`: introduce typed or categorized error objects/messages so callers can render contextual UI.
- `frontend/src/main.js`: map camera error categories to the existing `error-state` panel.
- `frontend/src/projection.js`: render the same contextual error when a USB camera fails in projection mode.
- `frontend/src/style.css`: minor additions for the error-state hint/call-to-action style if needed.
- No backend or Wails binding changes; no new dependencies.
