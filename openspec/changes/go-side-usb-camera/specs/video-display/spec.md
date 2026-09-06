## MODIFIED Requirements

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
