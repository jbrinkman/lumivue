## 1. Persisted scaling preference

- [x] 1.1 Add `videoScalingMode` to the Go configuration model, normalize missing or invalid values to `cover`, and add Go tests for defaulting plus JSON round-trip of `cover` and `contain`.
- [x] 1.2 Expose the scaling preference through the existing frontend configuration bindings and verify generated/runtime config objects preserve the field.

## 2. Shared video scaling presentation

- [x] 2.1 Add shared CSS presentation rules that apply `object-fit: cover` for Crop to Fill and `object-fit: contain` for Fit Inside to both USB and RTSP video elements in main and projection viewports; verify the frontend build succeeds.
- [x] 2.2 Apply the normalized saved mode when the main and projection views initialize and verify both source types use the same mode without changing stream URLs.

## 3. Toggle and cross-window synchronization

- [x] 3.1 Add a main-window scaling toggle with accessible text or state that identifies the active mode, and verify activating it switches between Crop to Fill and Fit Inside.
- [x] 3.2 Save each selected mode before applying it, retain the previous mode and surface an error if saving fails, and add tests for the persistence/update logic where test infrastructure supports it.
- [x] 3.3 Emit scaling-mode changes to the projection window and send the current mode when projection starts; verify an active projection updates without restarting its video stream.

## 4. Integration and regression verification

- [x] 4.1 Run `task check`, `task build`, and `go test -cover ./...`; verify all checks pass and touched Go application-logic packages retain at least 80% coverage.
- [x] 4.2 Manually verify Crop to Fill covers differently shaped main and projection viewports with proportional edge cropping for both USB and RTSP video.
- [x] 4.3 Manually verify Fit Inside shows the complete frame with proportional letterboxing or pillarboxing in both viewports and that resizing rescales the video without distortion.
- [x] 4.4 Restart the application after selecting each mode and verify the last selection is restored; verify a configuration with no preference defaults to Crop to Fill.
