## 1. macOS Release Packaging

- [x] 1.1 Replace the Wails v2-based packaging test with a failing test for the Wails v3 interface that asserts a version is required, `build/config.yml` receives that exact version, the Apple Silicon Wails v3 DMG task is used, and the output name includes Lumivue, version, platform, and architecture; verify the revised test fails for the missing Wails v3 implementation.
- [x] 1.2 Implement the Taskfile-backed unsigned DMG packaging command using the pinned Wails v3 CLI and project build tasks, then verify the focused packaging tests pass and a local macOS invocation produces a DMG containing `lumivue.app` with matching `CFBundleVersion` and `CFBundleShortVersionString` values.

## 2. Semantic Release Configuration

- [x] 2.1 Add failing configuration tests that assert the first release is exactly `1.0.0`, subsequent versions and notes follow Conventional Commits on `main`, packaging runs in `prepare` with `nextRelease.version`, and the GitHub publisher requires the versioned arm64 DMG asset; verify the tests fail before configuration exists.
- [x] 2.2 Add a private root Node package manifest, lockfile, age-vetted pinned Semantic Release dependencies, and release configuration satisfying the tests; verify `npm ci`, focused configuration tests, and non-publishing dry runs cover both initial `1.0.0` bootstrap and subsequent semantic versions.
- [x] 2.3 Add a failing test for the post-`1.0.0` no-release path using non-releasable commit fixtures, then complete any analyzer configuration needed and verify the test confirms no tag or GitHub release is requested.

## 3. Continuous Delivery Workflow

- [x] 3.1 Add failing workflow tests that require `workflow_dispatch` as the only trigger, `main` checkout with full history, repository verification before release, serialized release concurrency, an Apple Silicon macOS runner, installation of the pinned Wails v3 CLI matching `go.mod`, locked Node dependency installation, and only `contents: write` permission; verify the tests fail before the workflow exists.
- [x] 3.2 Correct the existing CI build and add the manually triggered CD workflow using the pinned Wails v3 CLI, then wire the repository token into Semantic Release; verify the workflow tests pass and action syntax validation reports no errors.
- [x] 3.3 Add workflow failure-path assertions proving push, schedule, and post-CI events cannot trigger publication and that verification or packaging failure stops execution before Semantic Release publication; verify all workflow tests pass.

## 4. Integrated Verification

- [x] 4.1 Run `task check`, `go test -cover ./...`, the release-tooling test suite, and Semantic Release dry run; verify all commands pass with no warnings and application-logic coverage remains at least 80%.
- [x] 4.2 Run `task build` and the Wails v3 macOS arm64 release packaging command with a test semantic version; verify the application builds without errors, the DMG filename is deterministic, and the bundled application reports the supplied version.
- [x] 4.3 Validate the OpenSpec change with strict validation and review the final workflow/configuration diff against every `release-delivery` scenario; verify no Windows, Linux, Intel, signing, notarization, changelog-commit, or npm-publication behavior was introduced.
