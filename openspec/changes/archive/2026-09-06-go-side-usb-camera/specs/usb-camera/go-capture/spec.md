## Purpose

Defines how the Go backend opens a USB camera device and serves the live feed to the frontend as an MJPEG stream, replacing the browser MediaDevices API inside the Wails WKWebView.

## ADDED Requirements

### Requirement: Start a USB camera capture session
The system SHALL start a Go-side capture for a configured USB camera source, return a localhost port serving an MJPEG stream, and keep the capture running until it is explicitly stopped.

#### Scenario: USB source activated
- **WHEN** the frontend requests playback of a configured USB camera source
- **THEN** the backend opens the matching AVFoundation device, begins decoding frames, and returns the port of an MJPEG HTTP endpoint

#### Scenario: Capture already running for the same source
- **WHEN** a USB camera source is already being captured and the frontend requests playback again
- **THEN** the system stops the existing capture and starts a fresh one, returning the new port

### Requirement: Stop a USB camera capture session
The system SHALL stop the capture for a given USB source and release the device and HTTP server resources when requested or when the application shuts down.

#### Scenario: User switches away from the USB source
- **WHEN** the active USB camera source is deselected or another source is selected
- **THEN** the backend stops the corresponding capture and the MJPEG endpoint becomes unavailable

#### Scenario: Application shuts down
- **WHEN** the Wails service shuts down
- **THEN** all active USB camera captures are stopped

### Requirement: Resolve a configured camera name to an AVFoundation device
The system SHALL map the configured camera name to an attached AVFoundation video capture device before opening it.

#### Scenario: Camera name matches an attached device
- **WHEN** the configured USB camera name is found among the currently attached AVFoundation video devices
- **THEN** the system opens that specific device for capture

#### Scenario: Camera name does not match any attached device
- **WHEN** the configured USB camera name cannot be matched to an attached AVFoundation video device
- **THEN** the system reports an error and does not start capture

### Requirement: Serve captured frames as an MJPEG stream
The system SHALL encode captured video frames as JPEG images and serve them over HTTP using `multipart/x-mixed-replace` so the frontend can render them like the existing RTSP relay.

#### Scenario: Frontend connects to the capture endpoint
- **WHEN** the frontend loads `http://127.0.0.1:<port>/stream`
- **THEN** it receives a continuous MJPEG stream until the capture stops or the client disconnects

### Requirement: Emit capture errors
The system SHALL emit a `usb:error` event when a USB camera capture fails for a recoverable or unrecoverable reason, and it SHALL stop the relay for that source on unrecoverable errors.

#### Scenario: Device is disconnected mid-capture
- **WHEN** the USB camera is physically disconnected while capturing
- **THEN** the system emits a `usb:error` event with the source ID and stops the capture

#### Scenario: Camera permission is denied at the OS level
- **WHEN** macOS denies camera access to the application
- **THEN** the system emits a `usb:error` event with a clear permission-denied message and stops the capture

### Requirement: Support only one active capture per source
The system SHALL allow only one active capture session per configured USB source, restarting cleanly when a new request is made for the same source.

#### Scenario: Same source activated twice
- **WHEN** the same USB source is activated a second time before the first capture has stopped
- **THEN** the first capture is stopped and a new capture is started, and only one MJPEG endpoint is active for that source
