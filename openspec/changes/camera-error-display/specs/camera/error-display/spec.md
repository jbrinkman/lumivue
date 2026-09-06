## Purpose

Defines how Lumivue surfaces USB camera playback failures to the user in a clear, actionable way, so that raw JavaScript errors are never shown in the main or projection window.

## ADDED Requirements

### Requirement: Categorized camera error messages
The system SHALL translate USB camera playback failures into user-facing error messages that clearly indicate the failure category instead of exposing raw exception text.

#### Scenario: WebView media API is unavailable
- **WHEN** the application tries to play a USB camera and `navigator.mediaDevices` is undefined
- **THEN** the error state SHALL display a message explaining that camera playback is not available in the current view, with a hint that a native implementation is required

#### Scenario: Camera permission is denied
- **WHEN** the application tries to play a USB camera and the operating system denies camera permission
- **THEN** the error state SHALL display a message stating that camera permission was denied and instruct the user to grant camera access in System Settings

#### Scenario: Selected camera is not found
- **WHEN** the application tries to play a USB camera whose deviceId cannot be matched to an attached camera
- **THEN** the error state SHALL display a message indicating the camera was not found and suggesting the user check that it is still connected

### Requirement: Main window camera error display
The system SHALL present categorized USB camera errors in the main window's existing error-state panel, replacing the empty-state content while the error is active.

#### Scenario: USB camera fails in main window
- **WHEN** a USB camera source fails to play in the main window
- **THEN** the empty-state panel SHALL be hidden, the error-state panel SHALL be shown with the categorized message, and the status overlay SHALL be cleared

#### Scenario: Error is cleared in main window
- **WHEN** the user deselects the failing source or selects a different source
- **THEN** the error-state panel SHALL be hidden and the empty-state panel SHALL be shown

### Requirement: Projection window camera error display
The system SHALL present the same categorized USB camera errors in the projection window when a USB camera source fails there.

#### Scenario: USB camera fails in projection window
- **WHEN** a USB camera source fails to play in the projection window
- **THEN** the projection window SHALL display the categorized error message in place of the "Waiting for source…" placeholder

#### Scenario: Projection error is cleared
- **WHEN** the failing source is replaced by another source or the projection is stopped
- **THEN** the projection window SHALL remove the error message and restore the appropriate empty or active state

### Requirement: Error messages are safe to render
The system SHALL escape or sanitize any dynamic values included in camera error messages before rendering them in the DOM.

#### Scenario: Camera name appears in error message
- **WHEN** an error message includes a camera name
- **THEN** the rendered text SHALL be escaped so that HTML or script content in the camera name cannot execute or alter the page layout
