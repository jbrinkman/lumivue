## Context

Lumivue uses Wails v3.0.0-beta.16 with a vanilla Vite frontend. Its existing CI still invokes the legacy Wails v2 CLI even though the Go and frontend dependencies use Wails v3, and the repository does not yet contain the Wails v3 Task-based build assets required for application and DMG packaging. The frontend has its own private npm package, so release automation should not treat that package version as the desktop application's source of truth. See `proposal.md` and `specs/release-delivery/spec.md` for scope and required behavior.

## Goals / Non-Goals

**Goals:**
- Keep release calculation, notes, tags, and GitHub release publication under one manually triggered Semantic Release lifecycle.
- Bootstrap the release history at `1.0.0`, then use Conventional Commits for all subsequent versions.
- Produce the versioned arm64 application and DMG before Semantic Release publishes the GitHub release.
- Make the packaging command reproducible locally on macOS with an explicit version input.
- Avoid persisting generated version changes back to `main`; the release tag is the durable version record.

**Non-Goals:**
- Managing the frontend package as a publishable npm package.
- Committing changelog or version-bump commits.
- Apple code signing, notarization, Intel/universal builds, or non-macOS packaging.
- Automatically releasing from branches other than `main`.

## Decisions

### Use repository-level Semantic Release tooling

Add a private root Node package manifest and lockfile containing pinned release development dependencies. Configure Semantic Release for `main` with commit analysis, release-note generation, an execution hook, and GitHub publication.

This separates delivery tooling from `frontend/package.json`, whose version describes a private build package rather than the desktop product. A global Semantic Release installation was rejected because it would make local and CI behavior less reproducible.

### Build during Semantic Release's prepare phase

Use the Semantic Release execution plugin to call a repository packaging command during `prepare`, passing the computed `nextRelease.version`. The command will:

1. update the Wails v3 `build/config.yml` version used to generate macOS metadata;
2. invoke the pinned Wails v3 Apple Silicon application and DMG packaging tasks;
3. verify both macOS bundle version fields resolve to the release version; and
4. place the unsigned DMG at a deterministic, versioned asset path consumed by the GitHub plugin.

Wails v3 delegates platform builds and packages to project Taskfiles and does not support the Wails v2 `wails build -platform` interface. The repository will adopt the Wails v3 build assets needed for `darwin:package:dmg` rather than maintaining a parallel hand-built application bundle. Building in `prepare` ensures failure prevents the publish phase and avoids a separate workflow trying to predict or rediscover Semantic Release's chosen version.

### Publish one versioned DMG through the GitHub plugin

Configure the GitHub plugin to upload the deterministic DMG as a release asset while it creates the tag and release notes. The asset name will include `lumivue`, the semantic version, `darwin`, and `arm64` so its platform and architecture are unambiguous.

A separate release-upload action was rejected because splitting tag/release creation and asset publication across tools increases the chance of incomplete releases.

### Run CD only through manual dispatch

Configure the CD workflow with `workflow_dispatch` as its only trigger, check out `main` with full history and tags, install the Wails v3 CLI version matching `go.mod`, and run release work on an Apple Silicon macOS runner. Grant workflow-level `contents: write` and no broader repository permissions. Repository permissions determine which authorized users may dispatch the workflow.

Manual dispatch gives maintainers explicit control over release timing. Automatic `push`, schedule, and post-CI `workflow_run` triggers were rejected because any of them could publish without an intentional release action. The workflow will run the repository verification commands before Semantic Release so a manually requested release cannot bypass the release gate.

### Bootstrap the first release at 1.0.0

Configure the initial release path so that, when no prior Lumivue semantic tag exists, a manual run establishes `1.0.0`. Once that tag exists, standard Semantic Release commit analysis determines all later increments and skips publication when no releasable commits exist.

Relying on Semantic Release's default first-version behavior was rejected because an existing commit history could otherwise select a version below or above the required `1.0.0` baseline. Manually creating the initial tag was also rejected because the first release must include the same generated notes, stamped application, and DMG publication lifecycle as later releases.

### Keep packaging independently testable

Expose packaging through the existing Taskfile with required version input and explicit output naming. Add automated checks for release configuration and packaging command construction where practical, plus a CI-safe dry-run or artifact inspection test that verifies the generated app metadata and DMG naming without publishing a release.

This follows the repository's test-driven workflow while recognizing that GitHub release publication itself is best validated through configuration tests and Semantic Release dry-run behavior rather than a real release in tests.

## Risks / Trade-offs

- [Unsigned applications trigger macOS trust warnings and may require users to bypass Gatekeeper] → Label release assets/notes as unsigned and keep signing/notarization as explicit future work.
- [Apple Silicon hosted runner availability or labels may change] → Use GitHub's supported arm64 macOS runner label at implementation time and pin the decision in the workflow.
- [A release plugin creates a tag before an asset upload failure] → Build and verify the DMG in `prepare`, before the publish lifecycle begins.
- [GitHub's default token can be blocked from creating releases by repository policy] → Use the least-privilege `contents: write` token and document repository policy as an operational prerequisite; do not add long-lived credentials.
- [Release tooling supply-chain exposure] → Commit a lockfile, use `npm ci`, and select dependency versions that satisfy the repository's dependency-age policy.
- [Wails v3 is a beta dependency and its build assets can change between releases] → Pin the CLI to the same version as `go.mod`, commit the project build assets, and verify packaging through tests rather than relying on globally installed tooling.
- [Two maintainers can manually request releases at nearly the same time] → Serialize release jobs with workflow concurrency so tag and version calculation cannot race.

## Migration Plan

1. Add the Wails v3 build assets and test the version-aware macOS arm64 DMG packaging command with the CLI pinned to the version in `go.mod`.
2. Add locked Semantic Release configuration and validate both the `1.0.0` bootstrap and subsequent-version paths with dry runs that cannot publish.
3. Correct CI/release automation to use the pinned Wails v3 CLI, then add the manually dispatched CD workflow with repository verification, serialized release concurrency, and least-privilege permissions.
4. After merge, an authorized maintainer manually dispatches the workflow; when no prior release exists, that run establishes version `1.0.0`.
5. For later releases, an authorized maintainer manually dispatches the workflow and Semantic Release determines whether Conventional Commits require a new version.
6. If rollback is required before publication, disable or revert the CD workflow. If publication has already occurred, preserve immutable release history and correct it with a subsequent semantic release rather than rewriting tags.
