## Purpose

Automated CI checks that run `go build`, `go vet`, a `gofmt` formatting check, `golangci-lint`, `govulncheck`, `actionlint` against the workflow itself, and race-tested tests on every push to `main` and on every pull request, giving contributors and reviewers a hands-off, automated signal against regressions. (Making this a required check via branch protection is a separate follow-up decision.)

## Requirements

### Requirement: CI Workflow Triggers on Push to Main and Pull Request
The system SHALL run the CI workflow automatically on every push to `main` and on every pull request, and SHALL cancel a superseded in-progress run for the same ref when a new run starts.

#### Scenario: Push to main triggers CI
- **WHEN** a commit is pushed to `main`
- **THEN** the CI workflow runs

#### Scenario: Pull request triggers CI
- **WHEN** a pull request is opened or updated
- **THEN** the CI workflow runs

#### Scenario: Superseded run is cancelled
- **WHEN** a new commit is pushed to a ref while an earlier CI run for that ref is still in progress
- **THEN** the earlier run is cancelled

### Requirement: Build Verification
The CI workflow SHALL run `go build ./...` and SHALL fail the workflow run if the build does not succeed.

#### Scenario: Build succeeds
- **WHEN** `go build ./...` completes without error
- **THEN** the build step passes

#### Scenario: Build fails
- **WHEN** `go build ./...` returns a non-zero exit code
- **THEN** the CI workflow run is marked failed

### Requirement: Vet Verification
The CI workflow SHALL run `go vet ./...` and SHALL fail the workflow run if vet reports any issues.

#### Scenario: Vet passes
- **WHEN** `go vet ./...` reports no issues
- **THEN** the vet step passes

#### Scenario: Vet fails
- **WHEN** `go vet ./...` reports one or more issues
- **THEN** the CI workflow run is marked failed

### Requirement: Formatting Verification
The CI workflow SHALL verify that all Go source files are formatted with `gofmt`, and SHALL fail the workflow run if any file is not.

#### Scenario: All files formatted
- **WHEN** `gofmt -l .` produces no output
- **THEN** the formatting check passes

#### Scenario: Unformatted file present
- **WHEN** `gofmt -l .` lists one or more files
- **THEN** the CI workflow run is marked failed

### Requirement: Lint Verification
The CI workflow SHALL run `golangci-lint run ./...` and SHALL fail the workflow run if it reports any issues.

#### Scenario: Lint passes
- **WHEN** `golangci-lint run ./...` reports no issues
- **THEN** the lint step passes

#### Scenario: Lint fails
- **WHEN** `golangci-lint run ./...` reports one or more issues
- **THEN** the CI workflow run is marked failed

### Requirement: Vulnerability Verification
The CI workflow SHALL run `govulncheck ./...` and SHALL fail the workflow run if it reports a known vulnerability.

#### Scenario: No vulnerabilities found
- **WHEN** `govulncheck ./...` reports no vulnerabilities
- **THEN** the vulnerability check passes

#### Scenario: Vulnerability found
- **WHEN** `govulncheck ./...` reports one or more known vulnerabilities
- **THEN** the CI workflow run is marked failed

### Requirement: Workflow Lint Verification
The CI workflow SHALL run `actionlint` against its own workflow files and SHALL fail the workflow run if it reports any issues.

#### Scenario: actionlint passes
- **WHEN** `actionlint` reports no issues
- **THEN** the workflow-lint step passes

#### Scenario: actionlint fails
- **WHEN** `actionlint` reports one or more issues
- **THEN** the CI workflow run is marked failed

### Requirement: Race-Tested Test Execution
The CI workflow SHALL run `go test -race ./...` and SHALL fail the workflow run if any test fails or the race detector reports a data race.

#### Scenario: Tests pass
- **WHEN** `go test -race ./...` completes with all tests passing and no race detected
- **THEN** the test step passes

#### Scenario: Test failure or race detected
- **WHEN** `go test -race ./...` reports a failing test or a data race
- **THEN** the CI workflow run is marked failed
