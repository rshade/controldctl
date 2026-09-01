## 1. Workflow Implementation

- [x] 1.1 Create `.github/workflows/ci.yml` triggered on `push` and `pull_request`, using `actions/checkout@v7` and `jdx/mise-action@v4` (`install_args: go`) to provision the toolchain, matching the convention already used in `.github/workflows/release.yml`; verify the file is valid YAML.
- [x] 1.2 Add workflow steps that run, in order, `go build ./...`, `go vet ./...`, `test -z "$(gofmt -l .)"`, and `go test -race ./...`; verify each step matches a requirement in `openspec/changes/archive/2026-08-31-add-ci-workflow/specs/ci-workflow/spec.md`.

## 2. Validation

- [x] 2.1 Run `actionlint .github/workflows/ci.yml` (available via `mise`) and verify it reports no errors.
- [x] 2.2 Run `go build ./...`, `go vet ./...`, `gofmt -l .`, and `go test -race ./...` locally against the current `main` branch and verify all four pass cleanly.
- [x] 2.3 ~~Open a pull request with the new workflow and verify the CI run appears in the GitHub Actions tab with all four steps (build, vet, gofmt, race tests) reporting success.~~ Verified via local dry-run instead (task 2.2) plus `actionlint` (task 2.1), per user decision to skip the live-PR check — no commit/push/PR was made as part of this change.
