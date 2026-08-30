## ADDED Requirements

### Requirement: Display all configured sources
The system SHALL present a switcher control that lists all configured sources by name.

#### Scenario: Switcher populated on launch
- **WHEN** the application starts with configured sources
- **THEN** all configured sources appear in the switcher control, each identified by its user-defined display name

#### Scenario: Switcher reflects config changes
- **WHEN** the user adds, removes, or renames a source in settings
- **THEN** the switcher updates immediately to reflect the new source set

### Requirement: Indicate the active source
The system SHALL visually distinguish the currently active source in the switcher.

#### Scenario: Active source highlighted
- **WHEN** a source is active and its video feed is displayed
- **THEN** that source's entry in the switcher is visually highlighted (e.g., border, background, or accent color) to distinguish it from inactive sources

#### Scenario: Active source updates on switch
- **WHEN** the user switches to a different source
- **THEN** the highlight moves to the newly active source within one render cycle

### Requirement: Switch active source with a single interaction
The system SHALL activate any configured source when the user selects it from the switcher, replacing the current video feed.

#### Scenario: User clicks a source in the switcher
- **WHEN** the user clicks or taps a source entry in the switcher
- **THEN** the system deactivates the current source and activates the selected one, updating the main video area

#### Scenario: Switch from USB to RTSP
- **WHEN** the user switches from an active USB camera to an RTSP stream
- **THEN** the USB MediaStream is stopped, the Go RTSP relay for the new stream is started, and the frontend renders the MJPEG feed

#### Scenario: Switch from RTSP to USB
- **WHEN** the user switches from an active RTSP stream to a USB camera
- **THEN** the Go RTSP relay is stopped, `getUserMedia` is called for the USB device, and the frontend renders the camera stream

### Requirement: Indicate the default source
The system SHALL visually mark the source designated as default in the switcher.

#### Scenario: Default source indicated
- **WHEN** a source is designated as the default
- **THEN** its switcher entry displays a visual marker (e.g., a star icon or "default" label) in addition to the active highlight when applicable

### Requirement: Support keyboard navigation in the switcher
The system SHALL allow the user to switch sources using the keyboard.

#### Scenario: Arrow key navigation
- **WHEN** the switcher has focus and the user presses the up or down arrow key
- **THEN** focus moves to the adjacent source entry in the list

#### Scenario: Enter to activate
- **WHEN** a source entry in the switcher has keyboard focus and the user presses Enter
- **THEN** that source is activated as if clicked
