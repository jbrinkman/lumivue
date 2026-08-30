## ADDED Requirements

### Requirement: Display active USB camera feed
The system SHALL render the live video feed of the active USB camera source inside the Wails application window using the browser's MediaDevices API.

#### Scenario: USB camera feed shown on activation
- **WHEN** a USB camera source is activated (on launch or via the switcher)
- **THEN** the system calls `getUserMedia` with the matching device ID and renders the stream in the main video area

#### Scenario: Camera permission granted
- **WHEN** the user grants camera permission when prompted by the OS
- **THEN** the video feed starts playing with no further user action required

#### Scenario: Camera permission denied
- **WHEN** the user denies camera permission
- **THEN** the system displays an error message explaining that camera access is required and how to grant it

### Requirement: Display active RTSP stream feed
The system SHALL relay an active RTSP stream through a Go-side decoder and serve it to the frontend as an MJPEG stream over a local HTTP server.

#### Scenario: RTSP feed shown on activation
- **WHEN** an RTSP stream source is activated
- **THEN** the Go backend opens the stream, begins decoding, and the frontend renders the MJPEG feed in the main video area

#### Scenario: RTSP stream unreachable
- **WHEN** the RTSP URL cannot be connected to within a 5-second timeout
- **THEN** the system displays an error state in the video area with the stream name and a retry button

#### Scenario: RTSP stream disconnects mid-session
- **WHEN** a connected RTSP stream drops unexpectedly
- **THEN** the system attempts one automatic reconnect after 3 seconds and displays a reconnecting indicator; if reconnect fails, shows the error state

### Requirement: Maintain aspect ratio
The system SHALL preserve the native aspect ratio of the video source when rendering in the video area.

#### Scenario: Video area wider than source aspect ratio
- **WHEN** the application window is wider than the source's native aspect ratio
- **THEN** the video is letterboxed (horizontal black bars) to fill the height while preserving width ratio

#### Scenario: Video area taller than source aspect ratio
- **WHEN** the application window is taller than the source's native aspect ratio
- **THEN** the video is pillarboxed (vertical black bars) to fill the width while preserving height ratio

### Requirement: Show empty state when no source is configured
The system SHALL display a placeholder UI when no sources are configured or no source is active.

#### Scenario: No sources configured
- **WHEN** the application launches and no sources have been configured
- **THEN** the video area shows a placeholder graphic and a prompt directing the user to open settings and configure a source

#### Scenario: No default set but sources exist
- **WHEN** the application launches and sources are configured but none is designated as default
- **THEN** the video area shows the first configured source in the list
