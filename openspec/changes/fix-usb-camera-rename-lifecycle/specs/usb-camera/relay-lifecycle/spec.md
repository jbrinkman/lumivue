## Purpose

The USB camera relay must start and stop cleanly, release FFmpeg resources only after the capture goroutine has finished, and fail gracefully when the requested camera cannot be found or opened.

## ADDED Requirements

### Requirement: Relay stop does not race with the capture goroutine
The system SHALL ensure that `USBCameraRelay.Stop()` waits for the `captureLoop` goroutine to exit before freeing any FFmpeg objects that the goroutine may still use.

#### Scenario: Switching between two USB cameras
- **WHEN** the user is playing one USB camera and selects a different USB camera
- **THEN** the system stops the first relay
- **THEN** the first relay's capture goroutine exits completely
- **THEN** the system frees the first relay's decoder and scaling context without panic
- **THEN** the system starts the second relay

#### Scenario: Starting the same USB source while it is already running
- **WHEN** the user triggers full-screen projection or otherwise re-selects the currently active USB camera
- **THEN** the system stops the existing relay and waits for its capture goroutine to exit
- **THEN** the system starts a new relay for the same source
- **THEN** no nil-pointer or use-after-free panic occurs

### Requirement: Start failure does not leave a half-initialized relay
The system SHALL return an error from `USBCameraRelay.Start` when device listing, camera resolution, or device opening fails, and SHALL NOT leave any goroutine or FFmpeg object allocated in a state that can cause a later crash.

#### Scenario: Camera name cannot be resolved
- **WHEN** `StartUSBCamera` is called for a source whose name does not match any system camera
- **THEN** `StartUSBCamera` returns an error to the frontend
- **THEN** no capture goroutine is left running
- **THEN** no nil-pointer panic occurs when the relay is later stopped or garbage collected

#### Scenario: Camera device cannot be opened
- **WHEN** the resolved camera device fails to open (e.g., already in use or permission denied)
- **THEN** `StartUSBCamera` returns an error to the frontend
- **THEN** the error is also emitted as a `usb:error` event
- **THEN** the application does not crash
