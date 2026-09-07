## Purpose

Let users give configured sources a custom label without changing the underlying identifier that the application uses to locate and start the source.

## ADDED Requirements

### Requirement: Renaming a source only changes its display label
The system SHALL allow the user to edit a display label for any configured source. For USB cameras the label MUST be separate from the system device name used for device resolution; for RTSP streams the label MUST be separate from the stream URL.

#### Scenario: USB camera is renamed and then selected
- **WHEN** the user renames a configured USB camera to a custom label
- **THEN** the settings list, sidebar, and status overlays display the custom label
- **WHEN** the user selects that source
- **THEN** the backend uses the original system device name to find and start the camera

#### Scenario: RTSP stream is renamed
- **WHEN** the user renames a configured RTSP stream
- **THEN** the settings list and sidebar display the new label
- **THEN** the backend continues to identify the stream by its URL and starts playback from that URL

### Requirement: Source ID remains stable after rename
The system SHALL NOT change a source's `id` when its display label is edited.

#### Scenario: Rename does not create a duplicate or orphaned source
- **WHEN** the user renames a source
- **THEN** the source keeps its original `id`
- **THEN** saving the configuration writes the same `id` with the updated label
