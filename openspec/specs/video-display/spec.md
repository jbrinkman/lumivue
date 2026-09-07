# Video Display Specification

## Purpose

Render the live video feed of the active source inside the main Wails window and in the fullscreen projection window with reliable, low-latency playback and clear error states.

## Requirements

### Requirement: Display active USB camera feed
The system SHALL render the live video feed of the active USB camera source inside the Wails application window via the Go-side MJPEG relay.

#### Scenario: USB camera feed shown on activation
- **WHEN** a USB camera source is activated (on launch or via the switcher)
- **THEN** the system starts the Go-side USB camera capture, sets the main video element to the MJPEG stream URL, and the feed renders in the main video area

#### Scenario: Camera permission granted
- **WHEN** the OS has already granted camera permission to the application
- **THEN** the video feed starts playing with no further user action required

#### Scenario: Camera permission denied
- **WHEN** macOS denies camera permission to the application
- **THEN** the system displays an error message explaining that camera access is required and how to grant it in System Settings

#### Scenario: USB camera capture fails
- **WHEN** the Go-side USB camera capture cannot start or stops unexpectedly
- **THEN** the system displays an error state in the main video area and stops trying to render that source

#### Scenario: Projection window renders the USB feed
- **WHEN** the active source is a USB camera and projection is active
- **THEN** the projection window also uses the Go-side MJPEG stream to render the feed

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
