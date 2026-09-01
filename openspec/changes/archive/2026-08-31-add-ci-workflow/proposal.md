## Why

Nothing currently enforces build, vet, gofmt, or test cleanliness on new commits — the initial implementation was manually re-verified by hand. Without an automated gate, a broken build, a vet failure, unformatted code, or a race condition can land on `main` unnoticed. This was flagged by the final whole-branch review of the initial implementation.

## What Changes

- Add a new GitHub Actions workflow (`.github/workflows/ci.yml`) that runs on `push` and `pull_request`.
- The workflow runs, in order: `go build ./...`, `go vet ./...`, a gofmt check (`test -z "$(gofmt -l .)"`), and `go test -race ./...`.
- Use the repo's existing `jdx/mise-action@v4` convention to provision the pinned Go toolchain (matches `release.yml`), so CI uses the same Go version declared in `mise.toml`.

## Capabilities

### New Capabilities
- `ci-workflow`: Automated CI checks (build, vet, gofmt, race-tested tests) that must pass on every push and pull request.

### Modified Capabilities
(none)

## Impact

- Affected code: new file `.github/workflows/ci.yml` only; no application code changes.
- CI/CD: adds a required check surface for future PRs (branch protection is not part of this change; enabling it as a required check is a follow-up decision for the repo owner).
- Dependencies: none added; reuses `jdx/mise-action@v4` and `actions/checkout` already used by `release.yml`.
