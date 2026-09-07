## Context

See `proposal.md` for motivation and `specs/video-display/spec.md` for required behavior. The main and projection views render USB and RTSP feeds in separate image elements. Configuration already persists application-wide preferences, and Wails events already synchronize source changes between windows.

## Goals / Non-Goals

**Goals:**
- Use one scaling implementation for USB and RTSP image elements.
- Keep the main and projection windows synchronized without restarting playback.
- Preserve compatibility with existing configuration files.

**Non-Goals:**
- Stretching video to a non-native aspect ratio.
- Pan, zoom, or user-selected crop positioning.
- Per-source or per-window scaling preferences.

## Decisions

- **Persist a `videoScalingMode` configuration value.** Allowed values are `cover` and `contain`, matching the underlying proportional scaling semantics. A missing or invalid value resolves to `cover`, preserving backward compatibility and implementing the initial Crop to Fill default. Alternatives such as browser-local storage were rejected because the Go-managed configuration is the existing application preference source.
- **Apply mode through a shared CSS class or data attribute on each video viewport.** Both USB and RTSP image elements use the same `object-fit` value: `cover` for Crop to Fill and `contain` for Fit Inside. CSS performs responsive rescaling when viewport dimensions change, avoiding JavaScript geometry calculations.
- **Expose one toggle in the main-window controls.** The control indicates the active mode and switches to the other mode when activated. Projection does not need a separate control because the preference is application-wide.
- **Synchronize projection with a Wails event.** On a successful mode change, the main window saves the updated configuration, applies it locally, and emits the new mode. The projection applies the event immediately. When projection opens, the main window includes or emits the current mode so a newly created projection cannot remain on the default accidentally.
- **Do not restart streams when scaling changes.** Changing presentation styling leaves image URLs and relay lifecycles untouched.

## Risks / Trade-offs

- [Risk] Saving fails after the main view changes visually. → Apply the mode after a successful save, or restore the previous mode and surface the save failure.
- [Risk] A projection window misses an event during startup. → Send the current mode when projection starts in addition to listening for later changes.
- [Risk] Crop to Fill hides meaningful content near frame edges. → The user can switch to Fit Inside at any time, and the preference is remembered.

## Migration Plan

Existing configurations require no migration. Absence of `videoScalingMode` is interpreted as Crop to Fill, and the field is written after the user first changes the mode.
