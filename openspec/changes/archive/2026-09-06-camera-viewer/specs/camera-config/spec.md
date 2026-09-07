## ADDED Requirements

### Requirement: Enumerate available USB cameras
The system SHALL detect and list all USB (and built-in) cameras currently connected to the host machine using the platform's native camera enumeration API.

#### Scenario: Cameras detected on launch
- **WHEN** the application starts
- **THEN** the system enumerates all currently attached camera devices and makes them available for configuration

#### Scenario: No cameras attached
- **WHEN** no camera devices are connected to the host
- **THEN** the system presents an empty camera list and prompts the user to connect a device or add an RTSP stream

### Requirement: Select USB cameras to include
The system SHALL allow the user to choose a named subset of the enumerated USB cameras to include in their configured source set.

#### Scenario: User includes a USB camera
- **WHEN** the user selects a USB camera from the enumerated list and confirms inclusion
- **THEN** the camera is added to the configured source set and appears in the camera switcher

#### Scenario: User excludes a USB camera
- **WHEN** the user deselects a previously included USB camera
- **THEN** the camera is removed from the configured source set and no longer appears in the camera switcher

#### Scenario: Built-in camera excluded by default
- **WHEN** the user opens the configuration screen for the first time
- **THEN** the built-in Mac camera is listed but NOT pre-selected, requiring an explicit opt-in

### Requirement: Add RTSP streams
The system SHALL allow the user to add RTSP streams by providing a display name and a stream URL.

#### Scenario: User adds a valid RTSP stream
- **WHEN** the user submits a name and a valid RTSP URL (beginning with `rtsp://` or `rtsps://`)
- **THEN** the stream is added to the configured source set and appears in the camera switcher

#### Scenario: User submits an invalid URL
- **WHEN** the user submits a URL that does not begin with `rtsp://` or `rtsps://`
- **THEN** the system rejects the entry and displays a validation error without saving

#### Scenario: Duplicate RTSP URL
- **WHEN** the user submits an RTSP URL that is already in the configured source set
- **THEN** the system warns the user and does not add a duplicate entry

### Requirement: Edit RTSP streams
The system SHALL allow the user to modify the name or URL of an existing RTSP stream entry.

#### Scenario: User edits an RTSP stream
- **WHEN** the user changes the name or URL of an existing RTSP stream and saves
- **THEN** the configured source set is updated to reflect the new name or URL

### Requirement: Remove configured sources
The system SHALL allow the user to remove any configured source (USB camera inclusion or RTSP stream) from the source set.

#### Scenario: User removes a source
- **WHEN** the user deletes a configured source
- **THEN** the source is removed from the source set and no longer appears in the camera switcher

#### Scenario: Removing the default source
- **WHEN** the user removes the source currently designated as default
- **THEN** the system clears the default designation and prompts the user to select a new default

### Requirement: Designate a default source
The system SHALL allow the user to designate exactly one configured source as the default.

#### Scenario: User sets a default
- **WHEN** the user marks a configured source as default
- **THEN** that source becomes the default and any previously set default is cleared

#### Scenario: App launches with a default set
- **WHEN** the application starts and a valid default source is configured
- **THEN** the default source is activated immediately and its video feed is displayed

### Requirement: Persist configuration
The system SHALL save the configured source set (USB selections, RTSP streams, default designation) to disk and restore it on next launch.

#### Scenario: Config survives restart
- **WHEN** the user quits and re-launches the application
- **THEN** all previously configured sources, names, and the default designation are restored exactly

#### Scenario: Config file location
- **WHEN** the configuration is written to disk
- **THEN** it is stored in the OS user-config directory (e.g., `~/Library/Application Support/lumivue/config.json` on macOS)
