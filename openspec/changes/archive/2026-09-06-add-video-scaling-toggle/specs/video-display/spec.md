## ADDED Requirements

### Requirement: Video preserves its aspect ratio while filling the viewport
The system SHALL scale every displayed video without altering its aspect ratio and SHALL support Crop to Fill and Fit Inside modes in both the main and projection viewports.

#### Scenario: Crop to Fill covers the viewport
- **WHEN** Crop to Fill mode is active and a video is displayed
- **THEN** the video scales proportionally until the viewport is fully covered
- **THEN** any overflow is cropped at the viewport edges
- **THEN** no empty letterbox or pillarbox area is visible

#### Scenario: Fit Inside shows the complete frame
- **WHEN** Fit Inside mode is active and a video is displayed
- **THEN** the video scales proportionally until the complete frame fits within the viewport
- **THEN** letterboxing or pillarboxing is visible when the video and viewport aspect ratios differ

#### Scenario: Viewport dimensions change
- **WHEN** the main or projection viewport is resized while video is displayed
- **THEN** the video is rescaled for the new viewport dimensions using the active mode
- **THEN** the video aspect ratio remains unchanged

### Requirement: Video scaling mode is shared and persisted
The system SHALL provide a control that switches between Crop to Fill and Fit Inside, SHALL apply the selected mode to both the main and projection viewports, and SHALL persist the selection.

#### Scenario: No preference has been saved
- **WHEN** the application loads without a saved video scaling preference
- **THEN** Crop to Fill is active

#### Scenario: User switches scaling mode
- **WHEN** the user activates the video scaling control
- **THEN** the mode switches between Crop to Fill and Fit Inside
- **THEN** all currently displayed main and projection video views update without restarting their streams
- **THEN** the selected mode is saved

#### Scenario: Saved preference is restored
- **WHEN** the application loads with a saved video scaling preference
- **THEN** that mode is applied to the main viewport
- **THEN** any projection viewport uses the same mode
