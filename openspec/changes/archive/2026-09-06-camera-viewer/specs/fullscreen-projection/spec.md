## ADDED Requirements

### Requirement: Enumerate connected monitors
The system SHALL enumerate all currently connected displays and expose them to the user for selection.

#### Scenario: Monitors listed in projection settings
- **WHEN** the user opens the fullscreen projection control
- **THEN** the system lists all connected monitors with their index and display name (e.g., "Display 1 – Built-in Retina Display", "Display 2 – Dell U2723D")

#### Scenario: Single monitor available
- **WHEN** only one monitor is connected
- **THEN** the system lists that monitor and allows projection to it (useful for testing), noting it will mirror the main window

### Requirement: Project active feed fullscreen on selected monitor
The system SHALL open a borderless, chrome-free window on the user-selected monitor that renders the same active video feed as the main window.

#### Scenario: User initiates fullscreen projection
- **WHEN** the user selects a monitor and triggers fullscreen projection
- **THEN** a new Wails window opens on the selected monitor in frameless fullscreen mode, displaying the active video feed

#### Scenario: Projection window has no chrome
- **WHEN** the fullscreen projection window is open
- **THEN** there is no title bar, window border, menu bar, or OS window decoration visible on the projected display

#### Scenario: Projection follows source switches
- **WHEN** the user switches the active source in the main window while projection is active
- **THEN** the projection window updates to show the new source within one second

### Requirement: Exit fullscreen projection
The system SHALL allow the user to close the fullscreen projection window and return to the normal windowed state.

#### Scenario: User closes projection from main window
- **WHEN** the user clicks a "Stop Projection" button or equivalent control in the main window
- **THEN** the fullscreen projection window is closed

#### Scenario: User presses Escape in the projection window
- **WHEN** the fullscreen projection window has focus and the user presses the Escape key
- **THEN** the fullscreen projection window is closed

#### Scenario: Projection window closed externally
- **WHEN** the OS closes the projection window (e.g., via Cmd+W or window management)
- **THEN** the main window's projection controls reset to "not projecting" state

### Requirement: Persist last-used monitor selection
The system SHALL remember the last monitor the user projected to and pre-select it on the next projection attempt.

#### Scenario: Last monitor pre-selected
- **WHEN** the user opens the fullscreen projection control after a previous projection session
- **THEN** the monitor used in the last session is pre-selected, if it is still connected

#### Scenario: Last monitor no longer connected
- **WHEN** the previously used monitor is no longer connected
- **THEN** the system falls back to the first available monitor with no error
