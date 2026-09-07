# Remove Source Specification

## Purpose

The settings panel must let users remove any configured video source (USB camera or RTSP stream) and have the updated source list persisted, without relying on browser-native dialogs that are unavailable or unreliable inside the Wails embedded webview.

## Requirements

### Requirement: Settings panel provides inline removal confirmation
The system SHALL display an inline confirmation control when the user chooses to remove a configured source, and SHALL complete the removal only after the user confirms.

#### Scenario: User confirms removal of a USB camera
- **WHEN** the user clicks the Remove button for a configured USB camera
- **THEN** the settings row switches to an inline confirmation state showing Confirm and Cancel actions
- **WHEN** the user clicks Confirm
- **THEN** the source is removed from the list and the new configuration is saved

#### Scenario: User cancels removal of an RTSP stream
- **WHEN** the user clicks the Remove button for a configured RTSP stream
- **THEN** the settings row switches to an inline confirmation state
- **WHEN** the user clicks Cancel
- **THEN** the confirmation state is dismissed and the source remains unchanged

### Requirement: Remove action works without native browser dialogs
The system SHALL NOT rely on `window.confirm()`, `window.prompt()`, or any other blocking browser dialog to perform removal or edit actions in the settings panel.

#### Scenario: Removing a source in the Wails webview
- **WHEN** the application is running inside the Wails WKWebView
- **THEN** clicking Remove and confirming still removes the source and saves the configuration

### Requirement: Removing the default source clears the default flag
The system SHALL ensure that removing the currently selected default source results in no source being marked as default, so the application does not attempt to start a non-existent source on launch.

#### Scenario: Default camera is removed
- **WHEN** the user removes the source that is currently marked as default
- **THEN** the source is removed and no remaining source has `isDefault: true`
