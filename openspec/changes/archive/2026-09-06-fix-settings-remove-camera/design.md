## Context

See `proposal.md` for the motivation. The settings panel in `frontend/src/settings.js` uses `window.confirm()` for remove confirmation and `window.prompt()` for RTSP stream renaming. In the Wails WKWebView these blocking dialogs are either non-functional or do not return the user's choice, which makes the buttons appear unresponsive. The USB rename flow already avoids `window.prompt()` by replacing the row with an inline edit form; the remove flow should follow the same pattern.

## Goals / Non-Goals

**Goals:**
- Make the Remove button for every source row work inside the Wails webview.
- Keep the confirmation state scoped to the individual source row (no global modal).
- Reuse the existing inline-edit pattern for RTSP renaming.
- Ensure the source list is saved and the settings panel re-renders after a successful removal.
- Handle removal of the currently active or default source gracefully.

**Non-Goals:**
- Adding a backend delete endpoint; source list updates continue to go through `SaveConfig(cfg)`.
- Changing the RTSP add flow.
- Adding undo functionality.

## Decisions

- **Inline per-row confirmation instead of a global dialog.** The existing `startInlineRename` already swaps a source row's content for an inline form. We will extend this pattern: clicking Remove replaces the row with a confirmation message and Confirm/Cancel buttons. This avoids any dependency on browser-native dialogs and keeps the UI state local.
- **Shared helper for confirm/cancel rendering.** A small `showRemoveConfirm(row, onConfirm, onCancel)` helper will be added to avoid duplicating the DOM manipulation for USB and RTSP rows.
- **RTSP edit converted to inline rename.** `handleRTSPEdit` currently uses `window.prompt()`. It will be changed to reuse the same inline rename path as USB sources, updating both the name and (when applicable) the URL. This is a minimal change because the rename infrastructure already exists.
- **Default source handling on removal.** When the removed source has `isDefault: true`, the remaining sources will naturally have `isDefault: false` because only one source can be default at a time. The design relies on the existing `SaveConfig` call to persist this state. If the removed source is the active playback source, `main.js`'s `handleConfigChange` already stops playback when the active source disappears from the config.
- **No Go backend changes.** The fix is confined to the frontend settings panel.

## Risks / Trade-offs

- [Risk] Inline confirmation could be accidentally confirmed if the user double-clicks. → Mitigation: keep the Confirm button disabled until the next animation frame or use `pointer-events` styling to avoid accidental double submission.
- [Risk] Removing the active source while it is playing may leave a stale `<img>` or `<video>` element. → Mitigation: `main.js` watches `handleConfigChange` and calls `deactivateSource` when the active source is removed. Verify during manual testing.
- [Risk] `window.prompt` removal for RTSP edit changes the interaction from a single prompt to an inline form. → Mitigation: keep the same two-button (Save/Cancel) pattern used for USB rename.

## Migration Plan

- No migration needed. Existing config files remain valid; this change only affects how the user edits them in the UI.

## Open Questions

- Should the Confirm button also stop an active stream before saving, or should `main.js`'s existing config-change handler continue to handle that?  
  _Resolved_: rely on the existing `handleConfigChange` path; verify manually.
