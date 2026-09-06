## Context

See `proposal.md` for motivation. The current flow is:

- `camera.js` throws plain `Error` strings from `playUSBCamera`.
- `main.js` catches the error and calls `showError(`Camera error: ${err.message}`)`.
- `projection.js` catches the error and sets `emptyEl.textContent = `Camera error: ${err.message}``.

There is no categorization, so the Wails WKWebView-specific failure (`navigator.mediaDevices` undefined) appears as an opaque technical message.

## Goals / Non-Goals

**Goals:**
- Categorize USB camera playback failures so the UI can show a contextual, user-friendly message.
- Keep all camera-error text in one place (the camera module) so main and projection windows stay consistent.
- Reuse the existing `error-state` DOM structure and `showError` helper in `main.js`.
- Ensure any dynamic values (camera names) are escaped before rendering.
- Avoid adding new dependencies or backend work.

**Non-Goals:**
- Fixing the underlying `navigator.mediaDevices` unavailability in Wails.
- Changing RTSP error handling, source configuration, or camera enumeration.
- Adding telemetry, retry logic, or automatic recovery.

## Decisions

### 1. Introduce a `CameraError` object in `camera.js`
`playUSBCamera` will reject with a small structured object containing `category` and `cameraName` instead of a plain `Error`. Callers receive enough information to render the right message without duplicating error text.

- **Why**: Categorizing by message string in every caller is brittle and duplicates logic.
- **Alternative considered**: Return a plain string category and require callers to maintain a message map. Rejected because centralizing the mapping in `camera.js` keeps the UI thin.
- **Shape**:
  ```js
  {
    category: 'api-unavailable' | 'permission-denied' | 'device-not-found' | 'unknown',
    cameraName: string,
    originalMessage?: string
  }
  ```

### 2. Add `getCameraErrorMessage(error)` helper in `camera.js`
This helper takes a `CameraError` and returns a user-facing object with `title` and `hint` fields. The main and projection windows use it directly.

- **Why**: One source of truth for user-facing copy; easy to update wording later.
- **Mapping**:
  - `api-unavailable` → title about camera access not being available, hint about native implementation being required.
  - `permission-denied` → title about permission denied, hint to check System Settings.
  - `device-not-found` → title about camera not being found, hint to check the connection.
  - `unknown` → title about a camera error, hint to check logs.

### 3. Update `main.js` to render categorized errors
`activateSource` will call `getCameraErrorMessage(err)` and pass the resulting `title` and `hint` to `showError`. `showError` will be extended to accept either a string (current behavior) or `{ title, hint }` and render the hint with a new CSS class if provided.

- **Why**: Minimal change to existing UI shell; backward-compatible with string errors.

### 4. Update `projection.js` to use the same helper
The projection window will call `getCameraErrorMessage(err)` and set `emptyEl.textContent` to `title + (hint ? '\n' + hint : '')`.

- **Why**: Keeps wording identical across windows. A simple text display is acceptable in projection mode because the panel is not interactive.
- **Escaping**: `textContent` is used, not `innerHTML`, so no HTML injection risk.

### 5. Escape camera names at the render site
`main.js` already has `escapeHtml`. The new `showError` path will escape the `title` and `hint` before inserting them. In `projection.js`, using `textContent` provides automatic escaping.

- **Why**: Camera names come from `system_profiler` and could contain unusual characters; rendering must be safe by default.

### 6. Minimal CSS addition
Add an `.error-state-hint` class in `style.css` for a smaller, muted line below the main error message. If the design can reuse existing variables and spacing without new rules, this can be skipped, but a dedicated class keeps the hint visually subordinate.

## Risks / Trade-offs

- [Risk] `playUSBCamera` currently throws plain `Error` instances. Changing the rejected value is a small API change, but all current callers are in this codebase and will be updated.
  - **Mitigation**: Update `main.js` and `projection.js` in the same change; search for other callers and update any tests.
- [Risk] Error categories are inferred from JavaScript API behavior, which can overlap (e.g., a permission denial may also surface as a generic `NotAllowedError`).
  - **Mitigation**: Map the specific error names/domains thrown by `getUserMedia` and `enumerateDevices` to the categories, falling back to `unknown` when uncertain.
- [Risk] The `api-unavailable` message tells the user a native implementation is needed, which may be interpreted as a promise of a future feature.
  - **Mitigation**: Keep the hint factual and neutral, e.g., "Camera playback is not available in this view. A native implementation would be required to enable it."
