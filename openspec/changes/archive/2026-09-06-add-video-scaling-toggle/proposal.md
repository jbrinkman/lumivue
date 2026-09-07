## Why

Video feeds do not consistently use the available viewport, leaving users without control over whether the full frame or the full display area takes priority. A shared scaling control will let users choose proportional cropping or proportional fitting in both the main and projection views.

## What Changes

- Scale every displayed video proportionally to the current viewport.
- Add a control that switches between Crop to Fill and Fit Inside modes.
- Default to Crop to Fill when no scaling preference has been saved.
- Persist the selected mode and apply it to both the main and projection viewports.
- Update an active projection when the scaling mode changes.

## Capabilities

### New Capabilities

- None.

### Modified Capabilities

- `video-display`: Define viewport-filling, aspect-ratio-preserving video scaling and a shared persisted mode toggle.

## Impact

- The persisted configuration gains a video scaling preference.
- Main-window controls and video styling change to expose and apply the scaling mode.
- Projection rendering listens for and applies the shared scaling preference.
- Configuration, frontend rendering, and cross-window event behavior require automated and manual verification.
