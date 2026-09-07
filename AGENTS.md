# Lumivue — Agent Guidelines

## Project Overview

Lumivue is a Go + Wails v2 desktop application with a vanilla JS / Vite frontend (no UI framework).
Build commands are managed via Taskfile (`task`). See `Taskfile.yaml` for all available tasks.

## Verification Commands

| Command | Purpose |
|---------|---------|
| `task fmt` | Format all Go source files |
| `task lint` | Run `go vet` |
| `task test` | Run all Go tests |
| `task check` | Run fmt, lint, and test together (use this before every commit) |
| `task build` | Build the application binary |

## Testing Requirements

- **Use test-driven development (red-green-refactor) for all code changes.**
  1. Write a failing test that expresses the required behavior before writing the production code (red).
  2. Implement the smallest amount of production code that makes the test pass (green).
  3. Refactor the implementation while keeping the test suite passing.
  4. Only then mark the implementation task complete.
- **No production code may be committed without a corresponding test**, unless the change is purely presentational, configuration-only, or documentation with no observable logic.
- **`task check` must pass before every commit** — no exceptions. This runs fmt, lint, and all tests.
- **No task may be marked complete while `task check` is failing.** Keep working until all checks pass, even if the failure was pre-existing before the current work began. We do not defer or skip pre-existing failures.
- **The application must build and run without errors after every commit.** Verify with `task build` when in doubt.
- **Warnings are treated as errors.** Address compiler warnings, vet warnings, and linter warnings immediately; do not commit code that produces warnings.
- **Application logic must maintain at least 80% unit and integration test coverage.** New Go code that contains application logic must be accompanied by tests that bring and keep coverage at or above this threshold. Use `go test -cover ./...` to verify coverage.
- **Tests must be meaningful.** Coverage from trivial or empty tests does not satisfy the 80% requirement. Each test must assert observable behavior.

## Commit Requirements

All commits must follow the [Conventional Commits](https://www.conventionalcommits.org/) format:

```
<type>[optional scope]: <description>

[optional body]

[optional footer(s)]
```

**Allowed types:**

| Type | Use for |
|------|---------|
| `feat` | A new feature or capability |
| `fix` | A bug fix |
| `test` | Adding or correcting tests (no production logic changes) |
| `refactor` | Code restructuring with no behavior change |
| `perf` | Performance improvements |
| `build` | Changes to build system, Taskfile, dependencies |
| `ci` | CI/CD configuration changes |
| `docs` | Documentation only |
| `chore` | Maintenance tasks that don't fit other types |
| `revert` | Reverting a previous commit |

**Rules:**
- The description must be lowercase and must not end with a period.
- Use the imperative mood in the description: "add camera switcher" not "added camera switcher".
- Scope is optional but encouraged for larger codebases: `feat(rtsp): add reconnect logic`.
- Breaking changes must include `!` after the type/scope (`feat!:`) and a `BREAKING CHANGE:` footer.
- For OpenSpec work, commit once at the end of the change, after all tasks in the spec are complete. Do not commit after individual tasks or groups of tasks.
- For work outside an OpenSpec change, commit once the full requested set of changes is complete and verified.
- Each commit must represent one logical change; do not bundle unrelated changes.
- `task check` must pass before the commit is made.

## General Development Guidelines

- **Task granularity**: each task must be completable and verifiable in isolation. Do not begin a task whose completion depends on a later task being done first. Do not bundle work from multiple tasks into one step. This controls implementation workflow, not commit frequency; commit after the full OpenSpec change or non-spec work request is complete.
- **Error handling**: Go errors must be wrapped with context using `fmt.Errorf("description: %w", err)`. Errors must never be silently swallowed — every error return must either be returned up the call stack, logged with context, or explicitly handled with a documented reason for discarding it.
- **No speculative code**: only implement what the current task explicitly requires. Do not add abstractions, helpers, configuration options, or features in anticipation of future tasks. Future tasks will add what they need when they need it.
