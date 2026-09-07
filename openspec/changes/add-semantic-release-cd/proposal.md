## Why

Lumivue currently validates macOS builds in CI but has no automated, repeatable way to version and publish installable releases. A continuous delivery workflow will turn conventional commit history into consistent versions, release notes, and downloadable macOS artifacts.

## What Changes

- Add a manually triggered Semantic Release workflow that creates GitHub releases only when explicitly dispatched.
- Set the first release version to `1.0.0`; derive every subsequent release version and its generated release notes from Conventional Commits.
- Propagate the derived semantic version into the Wails application metadata before packaging.
- Build an unsigned Apple Silicon macOS application and package it as a DMG attached to the GitHub release.
- Limit the initial release target to macOS arm64; Windows, Linux, Intel macOS, signing, and notarization remain out of scope.

## Capabilities

### New Capabilities
- `release-delivery`: Defines automated semantic versioning, release-note generation, application version stamping, and publication of a macOS arm64 DMG.

### Modified Capabilities

None.

## Impact

- Adds a GitHub Actions continuous delivery workflow and Semantic Release configuration.
- Adds Node-based release tooling and lockfile-managed development dependencies at the repository release-tooling boundary.
- Extends the Wails v3 Task-based production packaging path to accept the Semantic Release version and produce a DMG.
- Requires GitHub Actions permission to create tags/releases and upload release assets using the repository token.
- Does not add Apple signing credentials or change runtime application behavior.
