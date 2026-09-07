## Purpose

Defines how Lumivue versions and publishes reproducible GitHub releases with generated notes and an installable macOS artifact.

## ADDED Requirements

### Requirement: Release only when manually triggered
The release system SHALL begin release evaluation only when an authorized user manually dispatches the release workflow.

#### Scenario: Changes reach main without manual dispatch
- **WHEN** commits are added to `main` without a manual release workflow dispatch
- **THEN** the system does not evaluate or publish a release

#### Scenario: Authorized user requests a release
- **WHEN** an authorized user manually dispatches the release workflow
- **THEN** the system evaluates the repository state for release publication

### Requirement: Establish the initial release version
The release system SHALL assign version `1.0.0` when no prior Lumivue semantic release exists.

#### Scenario: First release is manually triggered
- **WHEN** an authorized user manually dispatches the release workflow and no prior Lumivue semantic release exists
- **THEN** the system creates release version `1.0.0`

### Requirement: Determine subsequent releases from conventional commits
After version `1.0.0` exists, the release system SHALL evaluate commits added since the latest release using Conventional Commits and SHALL create a subsequent release only when those commits require a semantic version increment.

#### Scenario: Releasable commits exist after the first release
- **WHEN** an authorized user manually dispatches the release workflow and unreleased commits require a semantic version increment
- **THEN** the system determines the next semantic version from those commits

#### Scenario: No releasable commits exist after the latest release
- **WHEN** an authorized user manually dispatches the release workflow and no unreleased commits require a semantic version increment
- **THEN** the system completes without creating a tag or GitHub release

### Requirement: Generate release notes
The release system SHALL generate release notes from the Conventional Commit history included in the release.

#### Scenario: Release notes describe included changes
- **WHEN** a new release is created
- **THEN** its GitHub release notes summarize the commits included since the previous release

### Requirement: Stamp the application version
The release system SHALL set the Wails application's user-visible macOS bundle version to the exact semantic version assigned to the release before building the application.

#### Scenario: Packaged version matches release
- **WHEN** the application is packaged for semantic version `X.Y.Z`
- **THEN** the macOS bundle reports version `X.Y.Z`

### Requirement: Publish a macOS installer
Each release SHALL include an unsigned DMG containing the Lumivue application built for Apple Silicon macOS.

#### Scenario: Successful release publication
- **WHEN** the release pipeline successfully creates a release
- **THEN** the corresponding GitHub release contains a downloadable arm64 DMG whose filename identifies Lumivue and the release version

#### Scenario: Packaging fails
- **WHEN** the application build or DMG packaging fails
- **THEN** the release pipeline fails and does not publish a successful GitHub release without the required installer

### Requirement: Restrict release publication permissions
The release workflow SHALL use repository-scoped automation credentials with only the permissions needed to read source contents and publish release tags, metadata, and assets.

#### Scenario: Release workflow requests permissions
- **WHEN** the continuous delivery workflow runs
- **THEN** its configured token permissions are limited to repository contents required for release publication
