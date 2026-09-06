## 1. Refactor `camera.js` error handling

- [x] 1.1 Introduce a `CameraError` object and update `playUSBCamera` to reject with `{ category, cameraName, originalMessage }` instead of plain `Error` strings. Verify by searching the frontend for remaining `throw new Error` calls inside `camera.js` and confirming the built frontend still compiles (`npm run build` in `frontend/`).
- [x] 1.2 Add `getCameraErrorMessage(error)` that maps `api-unavailable`, `permission-denied`, `device-not-found`, and `unknown` to `{ title, hint }`. Verify by calling the helper with each category in the dev console and checking that titles and hints are returned.

## 2. Update the main window error display

- [x] 2.1 Update `main.js` `activateSource` to call `getCameraErrorMessage(err)` and pass the result to `showError`. Extend `showError` to accept either a string or `{ title, hint }` and render the hint using `escapeHtml`. Verify by triggering a USB camera playback failure and confirming the main view shows the categorized title and hint, not the raw JavaScript message.
- [x] 2.2 Add an `.error-state-hint` style in `style.css` (or reuse existing muted text classes) so the hint is visually subordinate. Verify by inspecting the error-state panel and checking the hint color/ size.

## 3. Update the projection window error display

- [x] 3.1 Update `projection.js` to call `getCameraErrorMessage(err)` when `playUSBCamera` fails and set the projection empty-state text to the rendered title and hint. Use `textContent` to avoid HTML injection. Verify by starting projection with a failing USB camera and confirming the same categorized message appears in the projection window.

## 4. Final verification

- [x] 4.1 Run `task fmt`, `task lint`, and `task check` from the repo root. Confirm no warnings or errors.
- [x] 4.2 Run `task build` and confirm the application binary builds successfully. Manually exercise the USB camera path in both main and projection windows to confirm the error states for `api-unavailable`, `permission-denied`, and `device-not-found` are shown as designed.
