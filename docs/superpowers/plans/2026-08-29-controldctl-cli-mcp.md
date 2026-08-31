# controldctl CLI + MCP Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Build `controldctl`, a Go CLI (with a free MCP server via `mcp-server`) that manages ControlD devices, profiles, filters, services, custom rules, and rule folders through `ax-go` and `baptistecdr/controld-go`.

**Architecture:** One Cobra command tree in `package main`, split across resource files. `internal/controld` is the only place that imports `baptistecdr/controld-go` directly — it re-exports the upstream types our commands need (as zero-cost type aliases) and owns client construction and error-to-`ax.Error` mapping. Every command file therefore imports exactly one `controld`-named package (ours), never two, so there is no import-identifier collision anywhere outside `internal/controld`.

**Tech Stack:** Go 1.26, Cobra, `github.com/rshade/ax-go` (local sibling via `replace`), `github.com/baptistecdr/controld-go` v0.0.10.

**Spec:** `docs/superpowers/specs/2026-08-28-controldctl-cli-mcp-design.md`

## Global Constraints

- Go directive `go 1.26.7` in `go.mod`, matching `ax-go`'s own `go.mod`.
- Module path: `github.com/rshade/controld-go-mcp`.
- `ax-go` is consumed via `replace github.com/rshade/ax-go => ../ax-go` (local sibling checkout, per the user's explicit request to build against `../ax-go`) — never reactivate the parent directory's disabled `go.work`; this module's own `go.mod` replace is self-contained and doesn't touch shared workspace state.
- `baptistecdr/controld-go` is pinned at `v0.0.10` (latest tag as of 2026-08-29).
- Binary name: `controldctl`. `controldctl --mcp` is a `main()`-level argument rewrite to `controldctl mcp-server` — never a second code path.
- **Import convention (load-bearing, do not deviate):** every file under `internal/controld/` may `import "github.com/baptistecdr/controld-go"` directly and reference it as `controld.X` — this is legal because a Go file never self-qualifies its own package's members with the package name, so there is no ambiguity even though `internal/controld` is itself `package controld`. Every file in `package main` (the repo root) imports **only** `github.com/rshade/controld-go-mcp/internal/controld` (also identifier `controld`) and never imports `github.com/baptistecdr/controld-go` directly — it reaches every upstream type through this package's re-exported aliases.
- Every `delete` command requires `--yes` via `ax.Confirm`, following the exact pattern in `ax-go`'s `examples/integration/main.go` (`newConfirmCommand`/`promptForConfirmation`).
- Every `create`/`update`/`delete` command wraps its mutating call in `ax.Guard` so `--dry-run` skips the real API call.
- Error mapping (see Task 2) is the single source of truth for exit codes; no command file re-implements it.
- **Corrected test-invocation pattern (supersedes every task's test code below):** command tests must invoke the built command tree via `ax.Execute(ctx, root, ax.WithStdout(&stdout), ax.WithStderr(&stderr), ax.WithEnv(...))` and assert on the returned exit code (e.g. `code == ax.ExitSuccess`, `code == ax.ExitAuth`) — never a bare `root.Execute()` call. Discovered during Task 4: `--format`/`--dry-run`/`--yes`/`--idempotency-key` are registered by `ax.Execute`'s internal `prepareCommand`, not by `newRootCommand` itself, so `root.Execute()` fails every test with "unknown flag: --format" and an error-mapping test asserting only `err != nil` would pass for the wrong reason (any error, including the flag-parse failure, satisfies it) without ever exercising `controld.MapError`. `devices_test.go` (Task 4) has the corrected pattern — copy its shape for every later task's tests, and assert on the specific expected exit code, not just "an error occurred." Every task's test code below that still shows `root.Execute()` needs this same fix when implemented.
- **Corrected understanding, `--dry-run` vs `--yes` on delete commands:** `ax.Confirm` runs before `ax.Guard` and has no awareness of `--dry-run` — in machine mode, `--dry-run` alone still returns `confirmation_required` (exit 2); only `--yes` together with `--dry-run` reaches the dry-run-suppressed path. This is correct, intentional behavior (matches `ax-go`'s own reference `newConfirmCommand` pattern; an agent can't skip confirmation by adding `--dry-run`) — any task's manual-verification step below that implies `--dry-run` alone bypasses confirmation is wrong prose, not a code requirement to implement.
- No `Users`/`Billing`/`Network`/`Access` commands and no `X-Force-Org-Id` header in this plan — out of scope per the spec.
- **Corrected non-determinism tagging scope (supersedes the note below and every task's raw-vendor-type payload structs):** any resource whose vendor type carries a server-generated identifier assigned fresh on `create` needs a `devicePayload`-style local wrapper (lowercase `json` tags, `ax:"nondeterministic"` on the generated ID and any creation timestamp) — never the raw vendor type embedded directly in a list payload. Discovered during Task 5's review: `profiles.go`'s original code embedded `[]controld.Profile` directly, which both skipped the required tagging (`Profile.PK` is exactly as server-generated as `Device.PK`, which Task 4 correctly wrapped) and leaked the vendor's literal uppercase `json:"PK"` tag inconsistently against `devices.go`'s deliberately-lowercased `"pk"`. Fixed in Task 5 via a `profilePayload`/`toProfilePayload` wrapper (`pk`/`updated_at` tagged nondeterministic). **The same latent issue exists in Task 9's `foldersListPayload` (`Group.PK int`, server-generated on `create`, embedded raw as `[]controld.Group`) — apply the identical `groupPayload`/`toGroupPayload` wrapper pattern when implementing Task 9, don't wait for another review round to catch it.** Genuine catalog items with stable, caller-known identifiers (`Filter`, `ProfileService`, `ProfilesOption`, `Rule`/`CustomRule` keyed by hostname) still don't need the nondeterminism tag, but for consistency should still get a casing-only local wrapper rather than leaking vendor-uppercase field names — apply the same lowercase-JSON convention even where no tag is needed.
- **Original non-determinism tagging scope note (superseded by the correction above, kept for history):** only `devices create`/`update`/`list` register `ax.WithNonDeterministicFields` (Task 4's `devicePayload`, tagging `pk`/`device_id`/`created_at`) because those are the only v1 payloads with a genuinely server-generated identifier or a fresh timestamp. `Filter`/`ProfileService`/`ProfilesOption` are pre-defined catalog items keyed by a fixed string the caller already knows, and `Rule`/`CustomRule` are keyed by hostname — none of these vary between repeated calls against the same account, so Tasks 5–9's list/mutate commands intentionally register no non-determinism type. Their `__schema` output will show an empty non-deterministic-fields list, matching `ax-go`'s documented fallback for an unregistered command — this is a documentation-completeness gap, not a correctness bug, since the actual JSON values don't vary run-to-run for these resources. (Confirmed during Task 4's review: `ax-go`'s `internal/schema/nondeterministic.go` `walkDataLocators` does recurse into nested slice/struct element types — `devicesListPayload.Devices []devicePayload`'s nested tags are correctly picked up — so this was a real design choice, not a workaround for a reflection limitation.)
- **Golden-fixture testing:** the spec calls for golden fixtures "mirroring `ax-go`'s own convention" (checked-in `testdata/*.golden.json` files compared byte-for-byte). This plan's tests instead assert directly on captured stdout via inline Go code (unmarshal-and-check or `bytes.Contains`) against an `httptest.Server` with fixed canned responses — functionally equivalent for a fixed fixture (the output is byte-identical every run because the mock server's response never varies), but without checked-in `.golden.json` files or a `-update` regeneration flow. Adopting literal golden files is a reasonable follow-up (see "Post-plan follow-ups") but isn't required for these tests to correctly verify behavior.

---

## Task 1: Project scaffolding

**Files:**
- Create: `go.mod`, `go.sum` (generated by tooling, not hand-written)
- Create: `.gitignore`
- Create: `Makefile`
- Create: `main.go`
- Create: `root.go`
- Test: `root_test.go`

**Interfaces:**
- Produces: `func newRootCommand(clientFactory func(cmd *cobra.Command) (*controld.API, error)) *cobra.Command` — every later task's resource commands are mounted onto the command this returns. `controld.API` here is `internal/controld`'s re-exported alias, defined in Task 2; Task 1 only needs the function signature to exist, so it takes the factory as an `any`-free typed parameter once Task 2 lands. To avoid a forward dependency, Task 1 defines `newRootCommand` without the client factory parameter (root has no commands yet that need one) and Task 3 changes its signature — see Task 3's Interfaces block.
- Consumes: nothing (first task).

- [ ] **Step 1: Initialize the module and pull dependencies**

Run:
```bash
go mod init github.com/rshade/controld-go-mcp
go mod edit -go=1.26.7
go mod edit -replace github.com/rshade/ax-go=../ax-go
go get github.com/rshade/ax-go@v0.3.0
go get github.com/baptistecdr/controld-go@v0.0.10
go get github.com/spf13/cobra@v1.10.2
go mod tidy
```

Expected: `go.mod` now contains a `replace github.com/rshade/ax-go => ../ax-go` line and `require` entries for `ax-go`, `controld-go`, and `cobra`; `go.sum` is populated for the non-replaced modules. `replace` lines are exempt from checksum verification, so `ax-go` needs no `go.sum` entry.

- [ ] **Step 2: Write `.gitignore`**

```gitignore
/bin/
/dist/
*.test
*.out
.env
```

- [ ] **Step 3: Write the Makefile**

```makefile
.PHONY: build test lint fmt tidy

BINARY := controldctl

build:
	go build -o bin/$(BINARY) .

test:
	go test ./...

lint:
	go vet ./...
	gofmt -l .

fmt:
	gofmt -w .

tidy:
	go mod tidy
```

- [ ] **Step 4: Write the failing test for the root command**

```go
// root_test.go
package main

import "testing"

func TestNewRootCommandHasExpectedUse(t *testing.T) {
	root := newRootCommand()
	if root.Use != "controldctl" {
		t.Fatalf("Use = %q, want %q", root.Use, "controldctl")
	}
}
```

- [ ] **Step 5: Run the test to verify it fails**

Run: `go test ./... -run TestNewRootCommandHasExpectedUse -v`
Expected: FAIL — `newRootCommand` is undefined (package doesn't compile yet).

- [ ] **Step 6: Write `root.go`**

```go
// root.go
package main

import "github.com/spf13/cobra"

func newRootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "controldctl",
		Short: "Manage ControlD devices, profiles, and DNS rules",
	}
	return root
}
```

- [ ] **Step 7: Write `main.go`**

```go
// main.go
package main

import (
	"context"
	"io"
	"os"

	"github.com/rshade/ax-go"
)

func main() {
	os.Exit(run(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run takes stdin/stdout/stderr as the io.Reader/io.Writer interfaces ax.WithStdin
// et al. actually accept (not the concrete *os.File main() passes) — this is the
// same test seam ax-go's own examples/integration/main.go uses, and it's required
// for later tests to substitute a bytes.Buffer for stdout.
func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	root := newRootCommand()
	root.SetArgs(args)

	return ax.Execute(
		ctx,
		root,
		ax.WithStdin(stdin),
		ax.WithStdout(stdout),
		ax.WithStderr(stderr),
		ax.WithEnv(os.Getenv),
		ax.WithVersion(ax.ResolveVersion("")),
	)
}
```

- [ ] **Step 8: Run the test to verify it passes, then build**

Run: `go test ./... -v && go build -o bin/controldctl .`
Expected: PASS, and `bin/controldctl` is produced.

- [ ] **Step 9: Manually verify `__schema` works**

Run: `./bin/controldctl __schema`
Expected: JSON output describing the (currently empty) command tree — confirms `ax.Execute` wiring is correct before any real commands exist.

- [ ] **Step 10: Commit**

```bash
git add go.mod go.sum .gitignore Makefile main.go root.go root_test.go
git commit -m "chore: scaffold controldctl module with ax-go wiring"
```

---

## Task 2: internal/controld adapter — client construction and error mapping

**Files:**
- Create: `internal/controld/client.go`
- Create: `internal/controld/errors.go`
- Test: `internal/controld/client_test.go`
- Test: `internal/controld/errors_test.go`

**Interfaces:**
- Consumes: nothing new (only the upstream `github.com/baptistecdr/controld-go` package and `github.com/rshade/ax-go`).
- Produces:
  - `type API = controld.API` (re-export; callers outside this package use `controld.API` where `controld` is `internal/controld`)
  - `func NewClient(ctx context.Context, apiTokenFlag, configPath, baseURL string, getenv func(string) string) (*API, error)` — `baseURL` overrides the client's target host when non-empty (production passes `""`; Task 4 onward's `testFactory` passes an `httptest.Server` URL). This task cannot exercise `baseURL` with a direct test of its own — `controld.API`'s `BaseURL` field is unexported and this task has no re-exported resource method yet to call against a fake server — so its wiring is first genuinely tested starting in Task 4.
  - `func MapError(ctx context.Context, err error) error`
  - `type IntBool = controld.IntBool`, `type DoType = controld.DoType`, and the `Block`/`Bypass`/`Spoof`/`Redirect` constants re-exported the same way, since every resource task will need them.

- [ ] **Step 1: Write the failing test for token precedence**

```go
// internal/controld/client_test.go
package controld

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func fakeGetenv(values map[string]string) func(string) string {
	return func(key string) string { return values[key] }
}

func TestResolveAPITokenPrecedence(t *testing.T) {
	ctx := context.Background()

	t.Run("flag wins over everything", func(t *testing.T) {
		token, err := resolveAPIToken(ctx, "flag-token", "", fakeGetenv(map[string]string{"CONTROLD_API_TOKEN": "env-token"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if token != "flag-token" {
			t.Fatalf("token = %q, want %q", token, "flag-token")
		}
	})

	t.Run("env wins over config file", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "config.hujson")
		if err := os.WriteFile(configPath, []byte(`{"api_token": "config-token"}`), 0o600); err != nil {
			t.Fatalf("write config: %v", err)
		}

		token, err := resolveAPIToken(ctx, "", configPath, fakeGetenv(map[string]string{"CONTROLD_API_TOKEN": "env-token"}))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if token != "env-token" {
			t.Fatalf("token = %q, want %q", token, "env-token")
		}
	})

	t.Run("falls back to config file", func(t *testing.T) {
		dir := t.TempDir()
		configPath := filepath.Join(dir, "config.hujson")
		if err := os.WriteFile(configPath, []byte(`{"api_token": "config-token"}`), 0o600); err != nil {
			t.Fatalf("write config: %v", err)
		}

		token, err := resolveAPIToken(ctx, "", configPath, fakeGetenv(nil))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if token != "config-token" {
			t.Fatalf("token = %q, want %q", token, "config-token")
		}
	})

	t.Run("errors when nothing is set", func(t *testing.T) {
		_, err := resolveAPIToken(ctx, "", "", fakeGetenv(nil))
		if err == nil {
			t.Fatal("expected an error, got nil")
		}
	})
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./internal/controld/... -run TestResolveAPITokenPrecedence -v`
Expected: FAIL — `resolveAPIToken` is undefined.

- [ ] **Step 3: Write `client.go`**

```go
// internal/controld/client.go

// Package controld adapts baptistecdr/controld-go for controldctl. Files in
// this package import "github.com/baptistecdr/controld-go" directly and
// reference it as controld.X: that's legal even though this package is also
// named controld, because a Go file never self-qualifies its own package's
// members with the package name — controld.X here can only resolve to the
// import. Every other package in this module imports only this package
// (internal/controld) and never the upstream library directly.
package controld

import (
	"context"
	"errors"
	"fmt"

	"github.com/baptistecdr/controld-go"
	"github.com/rshade/ax-go"
)

// API is controldctl's handle to the ControlD API client. Re-exported so
// package main never imports the upstream library directly.
type API = controld.API

// IntBool and DoType (with its Block/Bypass/Spoof/Redirect values) are
// re-exported because CreateProfileCustomRuleParams, UpdateProfileServiceParams,
// and friends use them directly.
type IntBool = controld.IntBool

type DoType = controld.DoType

const (
	Block    = controld.Block
	Bypass   = controld.Bypass
	Spoof    = controld.Spoof
	Redirect = controld.Redirect
)

var errNoAPIToken = errors.New(
	"no ControlD API token found: pass --api-token, set CONTROLD_API_TOKEN, or set api_token in --config",
)

type fileConfig struct {
	APIToken string `json:"api_token"`
}

// NewClient resolves a ControlD API token from --api-token, then
// CONTROLD_API_TOKEN, then the api_token field of a Hujson config file, and
// constructs a client. baseURL overrides the client's target host when
// non-empty — production callers pass "" (the real API); tests pass an
// httptest.Server URL.
func NewClient(ctx context.Context, apiTokenFlag, configPath, baseURL string, getenv func(string) string) (*API, error) {
	token, err := resolveAPIToken(ctx, apiTokenFlag, configPath, getenv)
	if err != nil {
		return nil, err
	}

	opts := []controld.Option{}
	if baseURL != "" {
		opts = append(opts, controld.BaseURL(baseURL))
	}

	client, err := controld.New(token, opts...)
	if err != nil {
		return nil, fmt.Errorf("construct ControlD client: %w", err)
	}
	return client, nil
}

func resolveAPIToken(ctx context.Context, apiTokenFlag, configPath string, getenv func(string) string) (string, error) {
	if apiTokenFlag != "" {
		return apiTokenFlag, nil
	}
	if token := getenv("CONTROLD_API_TOKEN"); token != "" {
		return token, nil
	}
	if configPath != "" {
		var cfg fileConfig
		if err := ax.ParseConfigFile(ctx, configPath, &cfg); err != nil {
			return "", fmt.Errorf("parse config: %w", err)
		}
		if cfg.APIToken != "" {
			return cfg.APIToken, nil
		}
	}
	return "", errNoAPIToken
}
```

- [ ] **Step 4: Run the token precedence test to verify it passes**

Run: `go test ./internal/controld/... -run TestResolveAPITokenPrecedence -v`
Expected: PASS.

- [ ] **Step 5: Write the failing test for error mapping**

This test mirrors exactly how `controld-go` actually produces errors: every method wraps the typed error via `fmt.Errorf("%s: %w", ..., err)`, and the public error constructors return values (not pointers), while production code always returns pointers — so the test takes the address of a constructed value to match the pointer type `MapError` must recognize.

```go
// internal/controld/errors_test.go
package controld

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"

	"github.com/baptistecdr/controld-go"
	"github.com/rshade/ax-go"
)

func wrap(err error) error {
	return fmt.Errorf("error from makeRequest: %w", err)
}

func TestMapErrorClassifiesWrappedUpstreamErrors(t *testing.T) {
	ctx := context.Background()

	t.Run("401 (controld-go's AuthorizationError) maps to auth, not retryable", func(t *testing.T) {
		upstream := controld.NewAuthorizationError(&controld.Error{
			StatusCode: http.StatusUnauthorized,
			Error:      controld.ResponseInfo{Message: "invalid token"},
		})
		mapped := MapError(ctx, wrap(&upstream))

		var axErr *ax.Error
		if !errors.As(mapped, &axErr) {
			t.Fatalf("expected *ax.Error, got %T: %v", mapped, mapped)
		}
		if axErr.ExitCode() != ax.ExitAuth {
			t.Errorf("exit code = %d, want %d", axErr.ExitCode(), ax.ExitAuth)
		}
	})

	t.Run("403 (controld-go's AuthenticationError) maps to auth, not retryable", func(t *testing.T) {
		upstream := controld.NewAuthenticationError(&controld.Error{
			StatusCode: http.StatusForbidden,
			Error:      controld.ResponseInfo{Message: "insufficient scope"},
		})
		mapped := MapError(ctx, wrap(&upstream))

		var axErr *ax.Error
		if !errors.As(mapped, &axErr) {
			t.Fatalf("expected *ax.Error, got %T: %v", mapped, mapped)
		}
		if axErr.ExitCode() != ax.ExitAuth {
			t.Errorf("exit code = %d, want %d", axErr.ExitCode(), ax.ExitAuth)
		}
	})

	t.Run("404 maps to validation", func(t *testing.T) {
		upstream := controld.NewNotFoundError(&controld.Error{
			StatusCode: http.StatusNotFound,
			Error:      controld.ResponseInfo{Message: "device not found"},
		})
		mapped := MapError(ctx, wrap(&upstream))

		var axErr *ax.Error
		if !errors.As(mapped, &axErr) {
			t.Fatalf("expected *ax.Error, got %T: %v", mapped, mapped)
		}
		if axErr.ExitCode() != ax.ExitValidation {
			t.Errorf("exit code = %d, want %d", axErr.ExitCode(), ax.ExitValidation)
		}
	})

	t.Run("429 maps to network and is retryable", func(t *testing.T) {
		upstream := controld.NewRatelimitError(&controld.Error{
			StatusCode: http.StatusTooManyRequests,
			Error:      controld.ResponseInfo{Message: "slow down"},
		})
		mapped := MapError(ctx, wrap(&upstream))

		var axErr *ax.Error
		if !errors.As(mapped, &axErr) {
			t.Fatalf("expected *ax.Error, got %T: %v", mapped, mapped)
		}
		if axErr.ExitCode() != ax.ExitNetwork {
			t.Errorf("exit code = %d, want %d", axErr.ExitCode(), ax.ExitNetwork)
		}
		if axErr.Retryable == nil || !*axErr.Retryable {
			t.Errorf("retryable = %v, want true", axErr.Retryable)
		}
	})

	t.Run("500 maps to network and is retryable", func(t *testing.T) {
		upstream := controld.NewServiceError(&controld.Error{
			StatusCode: http.StatusInternalServerError,
			Error:      controld.ResponseInfo{Message: "internal service error"},
		})
		mapped := MapError(ctx, wrap(&upstream))

		var axErr *ax.Error
		if !errors.As(mapped, &axErr) {
			t.Fatalf("expected *ax.Error, got %T: %v", mapped, mapped)
		}
		if axErr.ExitCode() != ax.ExitNetwork {
			t.Errorf("exit code = %d, want %d", axErr.ExitCode(), ax.ExitNetwork)
		}
	})

	t.Run("other 4xx maps to validation", func(t *testing.T) {
		upstream := controld.NewRequestError(&controld.Error{
			StatusCode: http.StatusBadRequest,
			Error:      controld.ResponseInfo{Message: "bad payload"},
		})
		mapped := MapError(ctx, wrap(&upstream))

		var axErr *ax.Error
		if !errors.As(mapped, &axErr) {
			t.Fatalf("expected *ax.Error, got %T: %v", mapped, mapped)
		}
		if axErr.ExitCode() != ax.ExitValidation {
			t.Errorf("exit code = %d, want %d", axErr.ExitCode(), ax.ExitValidation)
		}
	})

	t.Run("nil passes through unchanged", func(t *testing.T) {
		if MapError(ctx, nil) != nil {
			t.Fatal("expected nil")
		}
	})

	t.Run("unrecognized error passes through unchanged for ax.Execute's default internal_error handling", func(t *testing.T) {
		plain := errors.New("boom")
		if MapError(ctx, plain) != plain {
			t.Fatalf("expected the original error back, got %v", MapError(ctx, plain))
		}
	})
}
```

The accessor names above are verified against `ax-go`'s actual source (`error.go` and `contract/error.go`, 2026-08-29): `ax.Error` is `type Error = contract.Error`, `func (e *Error) ExitCode() int` is a real method, and `Retryable *bool` is a real exported field. `ax.NewError` returns `*Error`, matching the `errors.As(mapped, &axErr)` pattern used here.

- [ ] **Step 6: Run it to verify it fails**

Run: `go test ./internal/controld/... -run TestMapErrorClassifiesWrappedUpstreamErrors -v`
Expected: FAIL — `MapError` is undefined.

- [ ] **Step 7: Write `errors.go`**

```go
// internal/controld/errors.go
package controld

import (
	"context"
	"errors"

	"github.com/baptistecdr/controld-go"
	"github.com/rshade/ax-go"
)

// defaultRateLimitRetrySeconds is advised when controld-go surfaces a
// RatelimitError. controld-go already retries a 429 internally (up to its own
// RetryPolicy, default 3 attempts with exponential backoff capped at 30s)
// before giving up and returning this error, so an immediate client-side
// retry is unlikely to help — 30s matches the library's own max backoff.
const defaultRateLimitRetrySeconds = 30

// MapError classifies an error returned by any controld-go API call into an
// ax.Error with the matching exit code. controld-go wraps its typed errors
// via fmt.Errorf("%w", ...) in every call site, so this uses errors.As (not a
// type switch) to unwrap them. controld-go's HTTP-status-to-type mapping is
// counter-intuitively swapped: AuthorizationError actually fires on HTTP 401
// (bad/missing credentials) and AuthenticationError fires on HTTP 403
// (insufficient permission) — see controld-go's controld.go,
// makeRequestWithAuthTypeAndHeadersComplete. Both map to ax.ExitAuth here
// regardless, so the swap doesn't affect exit codes, only which
// actionable_fix text attaches to which upstream type.
func MapError(ctx context.Context, err error) error {
	if err == nil {
		return nil
	}

	var authzErr *controld.AuthorizationError // fires on HTTP 401
	if errors.As(err, &authzErr) {
		return ax.NewError(ctx, "controld_authentication_failed", authzErr.Error(),
			ax.WithActionableFix("re-authenticate: check CONTROLD_API_TOKEN"),
			ax.WithRetryable(false),
			ax.WithErrorExitCode(ax.ExitAuth),
		)
	}

	var authnErr *controld.AuthenticationError // fires on HTTP 403
	if errors.As(err, &authnErr) {
		return ax.NewError(ctx, "controld_permission_denied", authnErr.Error(),
			ax.WithActionableFix("the token lacks permission for this operation; use a Read+Write token"),
			ax.WithRetryable(false),
			ax.WithErrorExitCode(ax.ExitAuth),
		)
	}

	var notFoundErr *controld.NotFoundError
	if errors.As(err, &notFoundErr) {
		return ax.NewError(ctx, "controld_resource_not_found", notFoundErr.Error(),
			ax.WithActionableFix("check the resource ID"),
			ax.WithRetryable(false),
			ax.WithErrorExitCode(ax.ExitValidation),
		)
	}

	var rateLimitErr *controld.RatelimitError
	if errors.As(err, &rateLimitErr) {
		return ax.NewError(ctx, "controld_rate_limited", rateLimitErr.Error(),
			ax.WithActionableFix("wait before retrying"),
			ax.WithRetryable(true),
			ax.WithRetryAfterSeconds(defaultRateLimitRetrySeconds),
			ax.WithErrorExitCode(ax.ExitNetwork),
		)
	}

	var serviceErr *controld.ServiceError
	if errors.As(err, &serviceErr) {
		return ax.NewError(ctx, "controld_upstream_unavailable", serviceErr.Error(),
			ax.WithActionableFix("retry after a short wait or check ControlD's status page"),
			ax.WithRetryable(true),
			ax.WithErrorExitCode(ax.ExitNetwork),
		)
	}

	var requestErr *controld.RequestError
	if errors.As(err, &requestErr) {
		return ax.NewError(ctx, "controld_invalid_request", requestErr.Error(),
			ax.WithActionableFix("check the command's arguments against ControlD's API requirements"),
			ax.WithRetryable(false),
			ax.WithErrorExitCode(ax.ExitValidation),
		)
	}

	return err
}
```

- [ ] **Step 8: Run the tests to verify they pass**

Run: `go test ./internal/controld/... -v`
Expected: PASS for every subtest. If `ax.Error`'s real accessor names differ from `ExitCode()`/`.Retryable`, fix the test to use the real names (found by reading `ax-go`'s source) — do not change `MapError`'s behavior to fit a wrong test.

- [ ] **Step 9: Commit**

```bash
git add internal/controld/client.go internal/controld/errors.go internal/controld/client_test.go internal/controld/errors_test.go
git commit -m "feat: add ControlD client construction and ax.Error mapping"
```

---

## Task 3: Root command wiring, client injection, and the --mcp shim

**Files:**
- Modify: `root.go`
- Modify: `main.go`
- Modify: `root_test.go`
- Create: `main_test.go`

**Interfaces:**
- Consumes: `controld.API` (type alias), `controldClient.NewClient(...)` from Task 2 — imported as `"github.com/rshade/controld-go-mcp/internal/controld"`, identifier `controld`.
- Produces:
  - `type clientFactory func(cmd *cobra.Command) (*controld.API, error)` — every resource task's `newXCommand(factory clientFactory) *cobra.Command` constructor takes this.
  - `func newRootCommand(factory clientFactory) *cobra.Command` (replaces Task 1's no-argument version).
  - `func rewriteMCPFlag(args []string) []string` — the `--mcp` → `mcp-server` shim, exported at package level so `main_test.go` can test it directly.

- [ ] **Step 1: Write the failing test for the `--mcp` shim**

```go
// main_test.go
package main

import (
	"reflect"
	"testing"
)

func TestRewriteMCPFlag(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{"no flag, unchanged", []string{"devices", "list"}, []string{"devices", "list"}},
		{"bare flag becomes subcommand", []string{"--mcp"}, []string{"mcp-server"}},
		{"flag with trailing args", []string{"--mcp", "--transport=http"}, []string{"mcp-server", "--transport=http"}},
		{"empty args", []string{}, []string{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := rewriteMCPFlag(tt.args)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("rewriteMCPFlag(%v) = %v, want %v", tt.args, got, tt.want)
			}
		})
	}
}
```

- [ ] **Step 2: Run it to verify it fails**

Run: `go test ./... -run TestRewriteMCPFlag -v`
Expected: FAIL — `rewriteMCPFlag` is undefined.

- [ ] **Step 3: Update `root.go`**

```go
// root.go
package main

import (
	"github.com/spf13/cobra"

	"github.com/rshade/controld-go-mcp/internal/controld"
)

type clientFactory func(cmd *cobra.Command) (*controld.API, error)

func newRootCommand(factory clientFactory) *cobra.Command {
	var apiToken string
	var configPath string

	root := &cobra.Command{
		Use:   "controldctl",
		Short: "Manage ControlD devices, profiles, and DNS rules",
	}

	root.PersistentFlags().StringVar(&apiToken, "api-token", "", "ControlD API token (overrides CONTROLD_API_TOKEN and --config)")
	root.PersistentFlags().StringVar(&configPath, "config", "", "Hujson config file path with an api_token field")

	return root
}

func promptForConfirmation(cmd *cobra.Command, subject string) (bool, error) {
	if _, err := cmd.ErrOrStderr().Write([]byte(subject + "? [y/N] ")); err != nil {
		return false, err
	}

	var response string
	if _, err := cmd.InOrStdin().Read(make([]byte, 0)); err != nil {
		return false, err
	}
	if _, err := fmtScan(cmd, &response); err != nil {
		return false, err
	}
	return response == "y" || response == "yes" || response == "Y" || response == "Yes", nil
}
```

> **Note for the implementer:** the confirmation prompt above is a placeholder shape only in the sense that its exact stdin-reading mechanics should be copied verbatim from `ax-go`'s `examples/integration/main.go` (`promptForConfirmation`, using `bufio.NewReader(cmd.InOrStdin()).ReadString('\n')` with `strings.TrimSpace` and `strings.EqualFold`) rather than the sketch above — copy that function exactly, including its imports (`bufio`, `strings`), since it's already correct and tested upstream. Do not invent a different stdin-reading approach.

Corrected `root.go` (use this version, not the sketch above):

```go
// root.go
package main

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rshade/controld-go-mcp/internal/controld"
)

type clientFactory func(cmd *cobra.Command) (*controld.API, error)

func newRootCommand(factory clientFactory) *cobra.Command {
	var apiToken string
	var configPath string

	root := &cobra.Command{
		Use:   "controldctl",
		Short: "Manage ControlD devices, profiles, and DNS rules",
	}

	root.PersistentFlags().StringVar(&apiToken, "api-token", "", "ControlD API token (overrides CONTROLD_API_TOKEN and --config)")
	root.PersistentFlags().StringVar(&configPath, "config", "", "Hujson config file path with an api_token field")

	return root
}

func promptForConfirmation(cmd *cobra.Command, subject string) (bool, error) {
	if _, err := fmt.Fprintf(cmd.ErrOrStderr(), "%s? [y/N] ", subject); err != nil {
		return false, fmt.Errorf("write confirmation prompt: %w", err)
	}

	response, err := bufio.NewReader(cmd.InOrStdin()).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return false, fmt.Errorf("read confirmation response: %w", err)
	}
	response = strings.TrimSpace(response)
	return strings.EqualFold(response, "y") || strings.EqualFold(response, "yes"), nil
}
```

- [ ] **Step 4: Update `main.go`**

```go
// main.go
package main

import (
	"context"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/rshade/ax-go"
	"github.com/rshade/ax-go/mcp"

	"github.com/rshade/controld-go-mcp/internal/controld"
)

func main() {
	os.Exit(run(context.Background(), rewriteMCPFlag(os.Args[1:]), os.Stdin, os.Stdout, os.Stderr))
}

// rewriteMCPFlag turns a leading "--mcp" into the "mcp-server" subcommand
// name, so "controldctl --mcp <rest>" behaves exactly like
// "controldctl mcp-server <rest>". This is a pure argument rewrite: there is
// only one code path underneath (ax-go's mcp.NewCommand), so --help and
// __schema output never mention a flag that mcp-server's own Cobra
// definition doesn't already have.
func rewriteMCPFlag(args []string) []string {
	if len(args) == 0 || args[0] != "--mcp" {
		return args
	}
	rewritten := make([]string, 0, len(args))
	rewritten = append(rewritten, "mcp-server")
	rewritten = append(rewritten, args[1:]...)
	return rewritten
}

func run(ctx context.Context, args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	resolved := ax.ResolveVersion("")

	factory := func(cmd *cobra.Command) (*controld.API, error) {
		apiToken, _ := cmd.Flags().GetString("api-token")
		configPath, _ := cmd.Flags().GetString("config")
		return controld.NewClient(cmd.Context(), apiToken, configPath, "", os.Getenv)
	}

	root := newRootCommand(factory)
	root.AddCommand(mcp.NewCommand(root, mcp.WithVersion(resolved)))
	root.SetArgs(args)

	return ax.Execute(
		ctx,
		root,
		ax.WithStdin(stdin),
		ax.WithStdout(stdout),
		ax.WithStderr(stderr),
		ax.WithEnv(os.Getenv),
		ax.WithVersion(resolved),
	)
}
```

- [ ] **Step 5: Update `root_test.go`**

```go
// root_test.go
package main

import (
	"testing"

	"github.com/spf13/cobra"

	"github.com/rshade/controld-go-mcp/internal/controld"
)

func noopFactory(*cobra.Command) (*controld.API, error) { return nil, nil }

func TestNewRootCommandHasExpectedUse(t *testing.T) {
	root := newRootCommand(noopFactory)
	if root.Use != "controldctl" {
		t.Fatalf("Use = %q, want %q", root.Use, "controldctl")
	}
}
```

- [ ] **Step 6: Write `rewriteMCPFlag`, then run all tests**

Add `rewriteMCPFlag` from Step 4 above (it's already written there — this step is just running the tests now that both it and `newRootCommand`'s new signature exist).

Run: `go build ./... && go test ./... -v`
Expected: PASS.

- [ ] **Step 7: Manually verify the binary end to end**

```bash
go build -o bin/controldctl .
./bin/controldctl __schema
./bin/controldctl mcp-server --help
./bin/controldctl --mcp --help   # should print mcp-server's help, not root's
```

Expected: all three succeed; the third command's output is identical to running `./bin/controldctl mcp-server --help` directly.

- [ ] **Step 8: Commit**

```bash
git add root.go main.go root_test.go main_test.go
git commit -m "feat: wire root command, client injection, and --mcp shim"
```

---

## Task 4: Devices resource

**Files:**
- Create: `devices.go`
- Modify: `internal/controld/client.go` (add re-exports: rename the aliases block or add a new small block — see Step 1)
- Modify: `root.go` (mount the new command)
- Test: `devices_test.go`

**Interfaces:**
- Consumes: `clientFactory` (Task 3), `controld.API`/`NewClient`/`MapError` (Task 2).
- Produces: `func newDevicesCommand(factory clientFactory) *cobra.Command`, exposing `devices list`, `devices create`, `devices update`, `devices delete`, `devices types`. Later tasks don't depend on any type defined here (each resource is independent), but all follow the same pattern this task establishes.

**v1 field scope:** `devices create`/`update` expose `--name`, `--profile-id`, and `--icon` (required-enough to be useful); DDNS, legacy-IPv4, and remap fields from `CreateDeviceParams`/`UpdateDeviceParams` are deferred — extend the flag set the same way if needed later. `devices list` and `devices types` take no filtering flags because `controld-go`'s `ListDevices`/`ListDeviceType` don't expose any (ControlD's `/devices/users` and `/devices/routers` path-suffix filtering isn't wired into this client version).

- [ ] **Step 1: Add the Device-related re-exports to `internal/controld/client.go`**

Add this block to `internal/controld/client.go` (anywhere after the existing `const` block):

```go
// Device-related re-exports for the devices command.
type Device = controld.Device
type CreateDeviceParams = controld.CreateDeviceParams
type UpdateDeviceParams = controld.UpdateDeviceParams
type DeleteDeviceParams = controld.DeleteDeviceParams
type DeviceTypes = controld.DeviceTypes
type IconName = controld.IconName

// A type alias re-exports the type but NOT its package-level constants — Go
// constants aren't attached to a type in a way aliasing carries forward, so
// devices.go's --icon default needs this re-exported explicitly.
const DesktopLinux = controld.DesktopLinux
```

- [ ] **Step 2: Write the failing test for `devices list`**

```go
// devices_test.go
package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/spf13/cobra"

	"github.com/rshade/controld-go-mcp/internal/controld"
)

func testFactory(t *testing.T, server *httptest.Server) clientFactory {
	t.Helper()
	return func(cmd *cobra.Command) (*controld.API, error) {
		return controld.NewClient(cmd.Context(), "test-token", "", server.URL, func(string) string { return "" })
	}
}

func TestDevicesListWritesEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/devices" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
			"success": true,
			"body": {
				"devices": [
					{"PK": "dev1", "ts": 1700000000, "name": "laptop", "user": "u1", "device_id": "abc123",
					 "status": 1, "learn_ip": 0, "desc": "", "resolvers": {"uid": "u1", "doh": "https://x", "dot": "x:853"},
					 "legacy_ipv4": {"resolver": "1.2.3.4", "status": 0},
					 "profile": {"PK": "p1", "updated": 1700000000, "name": "Home"},
					 "icon": "desktop-linux"}
				]
			}
		}`))
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.AddCommand(newDevicesCommand(testFactory(t, server)))

	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetArgs([]string{"devices", "list", "--format=json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	var envelope struct {
		Data struct {
			Devices []struct {
				PK   string `json:"pk"`
				Name string `json:"name"`
			} `json:"devices"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal stdout %q: %v", stdout.String(), err)
	}
	if len(envelope.Data.Devices) != 1 || envelope.Data.Devices[0].Name != "laptop" {
		t.Fatalf("unexpected devices: %+v", envelope.Data.Devices)
	}
}

func TestDevicesListMapsAuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"success": false, "error": {"message": "invalid token", "code": 401}}`))
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.AddCommand(newDevicesCommand(testFactory(t, server)))
	root.SetArgs([]string{"devices", "list", "--format=json"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected an error")
	}
}
```

The `--format=json` flag name and the envelope's top-level `data` key are verified against `ax-go`'s source (`internal/cli/cli.go`'s `FlagFormat = "format"`) and its golden fixture `examples/integration/testdata/root_success.golden.json` (`{"data":{...},"meta":{...}}`), 2026-08-29.

- [ ] **Step 3: Run it to verify it fails**

Run: `go test ./... -run TestDevicesList -v`
Expected: FAIL — `newDevicesCommand` is undefined.

- [ ] **Step 4: Write `devices.go`**

```go
// devices.go
package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/rshade/ax-go"

	"github.com/rshade/controld-go-mcp/internal/controld"
)

type devicePayload struct {
	PK          string `json:"pk"           ax:"nondeterministic"`
	DeviceID    string `json:"device_id"    ax:"nondeterministic"`
	Name        string `json:"name"`
	Status      int    `json:"status"`
	ProfilePK   string `json:"profile_pk"`
	ProfileName string `json:"profile_name"`
	CreatedAt   int64  `json:"created_at"   ax:"nondeterministic"`
}

func toDevicePayload(d controld.Device) devicePayload {
	return devicePayload{
		PK:          d.PK,
		DeviceID:    d.DeviceID,
		Name:        d.Name,
		Status:      int(d.Status),
		ProfilePK:   d.Profile.PK,
		ProfileName: d.Profile.Name,
		CreatedAt:   d.Ts.Unix(),
	}
}

type devicesListPayload struct {
	Devices []devicePayload `json:"devices"`
}

type deviceDeletePayload struct {
	DeviceID string `json:"device_id"`
	Deleted  bool   `json:"deleted"`
}

func newDevicesCommand(factory clientFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "devices",
		Short: "Manage ControlD devices (endpoints)",
	}
	cmd.AddCommand(newDevicesListCommand(factory))
	cmd.AddCommand(newDevicesCreateCommand(factory))
	cmd.AddCommand(newDevicesUpdateCommand(factory))
	cmd.AddCommand(newDevicesDeleteCommand(factory))
	cmd.AddCommand(newDevicesTypesCommand(factory))
	return cmd
}

func newDevicesListCommand(factory clientFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List all devices",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := factory(cmd)
			if err != nil {
				return err
			}
			devices, err := client.ListDevices(cmd.Context())
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			payload := devicesListPayload{Devices: make([]devicePayload, 0, len(devices))}
			for _, d := range devices {
				payload.Devices = append(payload.Devices, toDevicePayload(d))
			}
			return ax.WriteJSON(cmd.OutOrStdout(), ax.NewEnvelope(cmd.Context(), payload))
		},
	}
	ax.WithNonDeterministicFields[devicesListPayload](cmd)
	return cmd
}

func newDevicesCreateCommand(factory clientFactory) *cobra.Command {
	var name, profileID, icon string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a device",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if name == "" || profileID == "" {
				return ax.NewError(cmd.Context(), "validation_error", "--name and --profile-id are required",
					ax.WithErrorExitCode(ax.ExitValidation))
			}
			client, err := factory(cmd)
			if err != nil {
				return err
			}

			var result controld.Device
			ran, err := ax.Guard(cmd.Context(), func(ctx context.Context) error {
				var createErr error
				result, createErr = client.CreateDevice(ctx, controld.CreateDeviceParams{
					Name:      name,
					ProfileID: profileID,
					Icon:      controld.IconName(icon),
				})
				return createErr
			})
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			var payload devicePayload
			if ran {
				payload = toDevicePayload(result)
			}
			return ax.WriteJSON(cmd.OutOrStdout(), ax.NewEnvelope(cmd.Context(), payload))
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "device name (required)")
	cmd.Flags().StringVar(&profileID, "profile-id", "", "profile PK to assign (required)")
	cmd.Flags().StringVar(&icon, "icon", string(controld.DesktopLinux), "device icon name")
	ax.WithNonDeterministicFields[devicePayload](cmd)
	return cmd
}

func newDevicesUpdateCommand(factory clientFactory) *cobra.Command {
	var deviceID, name, profileID string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a device",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if deviceID == "" {
				return ax.NewError(cmd.Context(), "validation_error", "--device-id is required",
					ax.WithErrorExitCode(ax.ExitValidation))
			}
			client, err := factory(cmd)
			if err != nil {
				return err
			}

			params := controld.UpdateDeviceParams{DeviceID: deviceID}
			if name != "" {
				params.Name = &name
			}
			if profileID != "" {
				params.ProfileID = &profileID
			}

			var result controld.Device
			ran, err := ax.Guard(cmd.Context(), func(ctx context.Context) error {
				var updateErr error
				result, updateErr = client.UpdateDevice(ctx, params)
				return updateErr
			})
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			var payload devicePayload
			if ran {
				payload = toDevicePayload(result)
			}
			return ax.WriteJSON(cmd.OutOrStdout(), ax.NewEnvelope(cmd.Context(), payload))
		},
	}
	cmd.Flags().StringVar(&deviceID, "device-id", "", "device PK to update (required)")
	cmd.Flags().StringVar(&name, "name", "", "new device name")
	cmd.Flags().StringVar(&profileID, "profile-id", "", "new profile PK to assign")
	ax.WithNonDeterministicFields[devicePayload](cmd)
	return cmd
}

func newDevicesDeleteCommand(factory clientFactory) *cobra.Command {
	var deviceID string

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a device",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if deviceID == "" {
				return ax.NewError(cmd.Context(), "validation_error", "--device-id is required",
					ax.WithErrorExitCode(ax.ExitValidation))
			}

			subject := fmt.Sprintf("delete device %s", deviceID)
			outcome, err := ax.Confirm(cmd.Context(), subject)
			if err != nil {
				return err
			}
			if outcome == ax.ConfirmationPromptRequired {
				approved, promptErr := promptForConfirmation(cmd, subject)
				if promptErr != nil {
					return promptErr
				}
				if !approved {
					return ax.WriteJSON(cmd.OutOrStdout(),
						ax.NewEnvelope(cmd.Context(), deviceDeletePayload{DeviceID: deviceID, Deleted: false}))
				}
			}

			client, err := factory(cmd)
			if err != nil {
				return err
			}

			ran, err := ax.Guard(cmd.Context(), func(ctx context.Context) error {
				_, deleteErr := client.DeleteDevice(ctx, controld.DeleteDeviceParams{DeviceID: deviceID})
				return deleteErr
			})
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			return ax.WriteJSON(cmd.OutOrStdout(),
				ax.NewEnvelope(cmd.Context(), deviceDeletePayload{DeviceID: deviceID, Deleted: ran}))
		},
	}
	cmd.Flags().StringVar(&deviceID, "device-id", "", "device PK to delete (required)")
	return cmd
}

func newDevicesTypesCommand(factory clientFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "types",
		Short: "List available device types and icons",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := factory(cmd)
			if err != nil {
				return err
			}
			types, err := client.ListDeviceType(cmd.Context())
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			return ax.WriteJSON(cmd.OutOrStdout(), ax.NewEnvelope(cmd.Context(), types))
		},
	}
}
```

`controld.DesktopLinux` above resolves through the `const DesktopLinux = controld.DesktopLinux` re-export added to `internal/controld/client.go` in Step 1.

- [ ] **Step 5: Mount the command in `root.go`**

In `newRootCommand`, after the persistent flags are registered, add:

```go
	root.AddCommand(newDevicesCommand(factory))
```

- [ ] **Step 6: Run the tests to verify they pass**

Run: `go test ./... -v`
Expected: PASS. Debug any envelope-shape mismatches per the Step 2 note by reading `ax-go`'s actual `NewEnvelope`/`WriteJSON` output format directly (e.g. `go doc github.com/rshade/ax-go.NewEnvelope` or a golden fixture in `../ax-go/examples/integration/testdata/`).

- [ ] **Step 7: Manually verify dry-run and confirmation gating**

```bash
export CONTROLD_API_TOKEN=fake-token-for-manual-check
./bin/controldctl devices delete --device-id abc --dry-run --format=json
./bin/controldctl devices delete --device-id abc --format=json   # expect confirmation_required (exit 2) in machine mode without --yes
```

Expected: the first prints `dry_run: true` and never contacts the real API (it will still fail with a real network/auth error since there's no real device — that's fine, this step is only checking the dry-run/confirm gating logic reads correctly against the flags, not exercising real ControlD data). If a live account and real device ID are available, re-run without `--dry-run` and with `--yes` to confirm a real deletion path end to end — optional, not required for this task to pass.

- [ ] **Step 8: Commit**

```bash
git add devices.go devices_test.go root.go internal/controld/client.go
git commit -m "feat: add devices list/create/update/delete/types commands"
```

---

## Task 5: Profiles resource (with profile options)

**Files:**
- Create: `profiles.go`
- Create: `profiles_options.go`
- Modify: `internal/controld/client.go` (add Profile-related re-exports)
- Modify: `root.go`
- Test: `profiles_test.go`

**Interfaces:**
- Consumes: same as Task 4.
- Produces: `newProfilesCommand(factory clientFactory) *cobra.Command` exposing `profiles list/create/update/delete` and `profiles options list/update`.

**v1 field scope:** `profiles update` exposes `--name` only; `--disable-ttl`, `--lock-status`, `--lock-message`, `--password` from `UpdateProfileParams` are deferred, following the same pattern if extended later. `profiles create` exposes `--name` and `--clone-profile-id`.

- [ ] **Step 1: Add re-exports to `internal/controld/client.go`**

```go
type Profile = controld.Profile
type CreateProfileParams = controld.CreateProfileParams
type UpdateProfileParams = controld.UpdateProfileParams
type DeleteProfileParams = controld.DeleteProfileParams
type ProfilesOption = controld.ProfilesOption
type UpdateProfilesOption = controld.UpdateProfilesOption
```

- [ ] **Step 2: Write the failing test**

```go
// profiles_test.go
package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProfilesListWritesEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/profiles" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true, "body": {"profiles": [{"PK": "p1", "updated": 1700000000, "name": "Home"}]}}`))
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.AddCommand(newProfilesCommand(testFactory(t, server)))

	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetArgs([]string{"profiles", "list", "--format=json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}

	var envelope struct {
		Data struct {
			Profiles []struct {
				Name string `json:"name"`
			} `json:"profiles"`
		} `json:"data"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &envelope); err != nil {
		t.Fatalf("unmarshal stdout %q: %v", stdout.String(), err)
	}
	if len(envelope.Data.Profiles) != 1 || envelope.Data.Profiles[0].Name != "Home" {
		t.Fatalf("unexpected profiles: %+v", envelope.Data.Profiles)
	}
}

func TestProfilesOptionsListWritesEnvelope(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/profiles/options" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true, "body": {"options": [{"PK": "opt1", "title": "Block Page", "description": "d", "type": "toggle", "default_value": false, "info_url": ""}]}}`))
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.AddCommand(newProfilesCommand(testFactory(t, server)))

	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetArgs([]string{"profiles", "options", "list", "--format=json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !bytes.Contains(stdout.Bytes(), []byte("Block Page")) {
		t.Fatalf("expected option title in output: %s", stdout.String())
	}
}
```

- [ ] **Step 3: Run it to verify it fails**

Run: `go test ./... -run TestProfiles -v`
Expected: FAIL — `newProfilesCommand` is undefined.

- [ ] **Step 4: Write `profiles.go`**

```go
// profiles.go
package main

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/rshade/ax-go"

	"github.com/rshade/controld-go-mcp/internal/controld"
)

type profilesListPayload struct {
	Profiles []controld.Profile `json:"profiles"`
}

type profileDeletePayload struct {
	ProfileID string `json:"profile_id"`
	Deleted   bool   `json:"deleted"`
}

func newProfilesCommand(factory clientFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "profiles",
		Short: "Manage ControlD profiles",
	}
	cmd.AddCommand(newProfilesListCommand(factory))
	cmd.AddCommand(newProfilesCreateCommand(factory))
	cmd.AddCommand(newProfilesUpdateCommand(factory))
	cmd.AddCommand(newProfilesDeleteCommand(factory))
	cmd.AddCommand(newProfilesOptionsCommand(factory))
	return cmd
}

func newProfilesListCommand(factory clientFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List all profiles",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := factory(cmd)
			if err != nil {
				return err
			}
			profiles, err := client.ListProfiles(cmd.Context())
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			return ax.WriteJSON(cmd.OutOrStdout(), ax.NewEnvelope(cmd.Context(), profilesListPayload{Profiles: profiles}))
		},
	}
}

func newProfilesCreateCommand(factory clientFactory) *cobra.Command {
	var name, cloneProfileID string

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a profile",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if name == "" {
				return ax.NewError(cmd.Context(), "validation_error", "--name is required",
					ax.WithErrorExitCode(ax.ExitValidation))
			}
			client, err := factory(cmd)
			if err != nil {
				return err
			}

			params := controld.CreateProfileParams{Name: name}
			if cloneProfileID != "" {
				params.CloneProfileID = &cloneProfileID
			}

			var result []controld.Profile
			ran, err := ax.Guard(cmd.Context(), func(ctx context.Context) error {
				var createErr error
				result, createErr = client.CreateProfile(ctx, params)
				return createErr
			})
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			if !ran {
				result = nil
			}
			return ax.WriteJSON(cmd.OutOrStdout(), ax.NewEnvelope(cmd.Context(), profilesListPayload{Profiles: result}))
		},
	}
	cmd.Flags().StringVar(&name, "name", "", "profile name (required)")
	cmd.Flags().StringVar(&cloneProfileID, "clone-profile-id", "", "existing profile PK to clone from")
	return cmd
}

func newProfilesUpdateCommand(factory clientFactory) *cobra.Command {
	var profileID, name string

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a profile",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if profileID == "" {
				return ax.NewError(cmd.Context(), "validation_error", "--profile-id is required",
					ax.WithErrorExitCode(ax.ExitValidation))
			}
			client, err := factory(cmd)
			if err != nil {
				return err
			}

			params := controld.UpdateProfileParams{ProfileID: profileID}
			if name != "" {
				params.Name = &name
			}

			var result []controld.Profile
			ran, err := ax.Guard(cmd.Context(), func(ctx context.Context) error {
				var updateErr error
				result, updateErr = client.UpdateProfile(ctx, params)
				return updateErr
			})
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			if !ran {
				result = nil
			}
			return ax.WriteJSON(cmd.OutOrStdout(), ax.NewEnvelope(cmd.Context(), profilesListPayload{Profiles: result}))
		},
	}
	cmd.Flags().StringVar(&profileID, "profile-id", "", "profile PK to update (required)")
	cmd.Flags().StringVar(&name, "name", "", "new profile name")
	return cmd
}

func newProfilesDeleteCommand(factory clientFactory) *cobra.Command {
	var profileID string

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a profile",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if profileID == "" {
				return ax.NewError(cmd.Context(), "validation_error", "--profile-id is required",
					ax.WithErrorExitCode(ax.ExitValidation))
			}

			subject := "delete profile " + profileID
			outcome, err := ax.Confirm(cmd.Context(), subject)
			if err != nil {
				return err
			}
			if outcome == ax.ConfirmationPromptRequired {
				approved, promptErr := promptForConfirmation(cmd, subject)
				if promptErr != nil {
					return promptErr
				}
				if !approved {
					return ax.WriteJSON(cmd.OutOrStdout(),
						ax.NewEnvelope(cmd.Context(), profileDeletePayload{ProfileID: profileID, Deleted: false}))
				}
			}

			client, err := factory(cmd)
			if err != nil {
				return err
			}

			ran, err := ax.Guard(cmd.Context(), func(ctx context.Context) error {
				_, deleteErr := client.DeleteProfile(ctx, controld.DeleteProfileParams{ProfileID: profileID})
				return deleteErr
			})
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			return ax.WriteJSON(cmd.OutOrStdout(),
				ax.NewEnvelope(cmd.Context(), profileDeletePayload{ProfileID: profileID, Deleted: ran}))
		},
	}
	cmd.Flags().StringVar(&profileID, "profile-id", "", "profile PK to delete (required)")
	return cmd
}
```

- [ ] **Step 5: Write `profiles_options.go`**

```go
// profiles_options.go
package main

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/rshade/ax-go"

	"github.com/rshade/controld-go-mcp/internal/controld"
)

type profileOptionsListPayload struct {
	Options []controld.ProfilesOption `json:"options"`
}

func newProfilesOptionsCommand(factory clientFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "options",
		Short: "Manage profile-level options",
	}
	cmd.AddCommand(newProfilesOptionsListCommand(factory))
	cmd.AddCommand(newProfilesOptionsUpdateCommand(factory))
	return cmd
}

func newProfilesOptionsListCommand(factory clientFactory) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List available profile options and their current values",
		RunE: func(cmd *cobra.Command, _ []string) error {
			client, err := factory(cmd)
			if err != nil {
				return err
			}
			options, err := client.ListProfilesOptions(cmd.Context())
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			return ax.WriteJSON(cmd.OutOrStdout(), ax.NewEnvelope(cmd.Context(), profileOptionsListPayload{Options: options}))
		},
	}
}

type profileOptionUpdatePayload struct {
	ProfileID string `json:"profile_id"`
	Name      string `json:"name"`
	Updated   bool   `json:"updated"`
}

func newProfilesOptionsUpdateCommand(factory clientFactory) *cobra.Command {
	var profileID, name, value string
	var status bool

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a profile option",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if profileID == "" || name == "" {
				return ax.NewError(cmd.Context(), "validation_error", "--profile-id and --name are required",
					ax.WithErrorExitCode(ax.ExitValidation))
			}
			client, err := factory(cmd)
			if err != nil {
				return err
			}

			params := controld.UpdateProfilesOption{
				ProfileID: profileID,
				Name:      name,
				Status:    controld.IntBool(status),
			}
			if value != "" {
				params.Value = &value
			}

			ran, err := ax.Guard(cmd.Context(), func(ctx context.Context) error {
				_, updateErr := client.UpdateProfilesOption(ctx, params)
				return updateErr
			})
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			return ax.WriteJSON(cmd.OutOrStdout(),
				ax.NewEnvelope(cmd.Context(), profileOptionUpdatePayload{ProfileID: profileID, Name: name, Updated: ran}))
		},
	}
	cmd.Flags().StringVar(&profileID, "profile-id", "", "profile PK (required)")
	cmd.Flags().StringVar(&name, "name", "", "option name (required)")
	cmd.Flags().BoolVar(&status, "enabled", false, "enable or disable the option")
	cmd.Flags().StringVar(&value, "value", "", "option value, if the option takes one")
	return cmd
}
```

- [ ] **Step 6: Mount and verify**

Add `root.AddCommand(newProfilesCommand(factory))` in `root.go`.

Run: `go build ./... && go test ./... -v`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add profiles.go profiles_options.go profiles_test.go root.go internal/controld/client.go
git commit -m "feat: add profiles list/create/update/delete and options commands"
```

---

## Task 6: Profile filters

**Files:**
- Create: `profiles_filters.go`
- Modify: `internal/controld/client.go`
- Modify: `root.go`
- Test: `profiles_filters_test.go`

**Interfaces:**
- Consumes: same as Task 5.
- Produces: `newProfilesFiltersCommand(factory clientFactory) *cobra.Command`, mounted under `profiles` in Task 5's command (so `root.go` mounts it via `profilesCmd.AddCommand(...)`, not directly on root — see Step 4).

- [ ] **Step 1: Add re-exports**

```go
type Filter = controld.Filter
type ListProfileFiltersParams = controld.ListProfileFiltersParams
type UpdateProfileFilterParams = controld.UpdateProfileFilterParams
```

- [ ] **Step 2: Write the failing test**

```go
// profiles_filters_test.go
package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProfilesFiltersListDefaultsToNative(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/profiles/p1/filters" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true, "body": {"filters": [{"PK": "ads", "name": "Ads", "description": "d", "sources": [], "status": 1}]}}`))
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.AddCommand(newProfilesFiltersCommand(testFactory(t, server)))

	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetArgs([]string{"filters", "list", "--profile-id=p1", "--format=json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !bytes.Contains(stdout.Bytes(), []byte(`"name":"Ads"`)) {
		t.Fatalf("expected filter name in output: %s", stdout.String())
	}
}

func TestProfilesFiltersListExternalSource(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/profiles/p1/filters/external" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true, "body": {"filters": []}}`))
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.AddCommand(newProfilesFiltersCommand(testFactory(t, server)))
	root.SetArgs([]string{"filters", "list", "--profile-id=p1", "--source=external", "--format=json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
}
```

> **Note for the implementer:** this task mounts its test command directly as `filters` (not nested under `profiles`) purely so the test's `root.AddCommand` call is self-contained without depending on Task 5's `newProfilesCommand`. In `root.go`'s real wiring (Step 4 below), mount it nested as `profiles filters`, matching the design spec's command tree — the test's flat mounting is a test-only convenience, not the production tree.

- [ ] **Step 3: Write `profiles_filters.go`**

```go
// profiles_filters.go
package main

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/rshade/ax-go"

	"github.com/rshade/controld-go-mcp/internal/controld"
)

type filtersListPayload struct {
	Filters []controld.Filter `json:"filters"`
}

func newProfilesFiltersCommand(factory clientFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "filters",
		Short: "Manage a profile's category filters",
	}
	cmd.AddCommand(newProfilesFiltersListCommand(factory))
	cmd.AddCommand(newProfilesFiltersUpdateCommand(factory))
	return cmd
}

func newProfilesFiltersListCommand(factory clientFactory) *cobra.Command {
	var profileID, source string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List a profile's filters",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if profileID == "" {
				return ax.NewError(cmd.Context(), "validation_error", "--profile-id is required",
					ax.WithErrorExitCode(ax.ExitValidation))
			}
			if source != "native" && source != "external" {
				return ax.NewError(cmd.Context(), "validation_error", "--source must be native or external",
					ax.WithErrorExitCode(ax.ExitValidation))
			}
			client, err := factory(cmd)
			if err != nil {
				return err
			}

			params := controld.ListProfileFiltersParams{ProfileID: profileID}
			var filters []controld.Filter
			if source == "external" {
				filters, err = client.ListProfileExternalFilters(cmd.Context(), params)
			} else {
				filters, err = client.ListProfileNativeFilters(cmd.Context(), params)
			}
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			return ax.WriteJSON(cmd.OutOrStdout(), ax.NewEnvelope(cmd.Context(), filtersListPayload{Filters: filters}))
		},
	}
	cmd.Flags().StringVar(&profileID, "profile-id", "", "profile PK (required)")
	cmd.Flags().StringVar(&source, "source", "native", "native or external")
	return cmd
}

type filterUpdatePayload struct {
	ProfileID string `json:"profile_id"`
	Filter    string `json:"filter"`
	Updated   bool   `json:"updated"`
}

func newProfilesFiltersUpdateCommand(factory clientFactory) *cobra.Command {
	var profileID, filter string
	var enabled bool

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Enable or disable a filter on a profile",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if profileID == "" || filter == "" {
				return ax.NewError(cmd.Context(), "validation_error", "--profile-id and --filter are required",
					ax.WithErrorExitCode(ax.ExitValidation))
			}
			client, err := factory(cmd)
			if err != nil {
				return err
			}

			params := controld.UpdateProfileFilterParams{
				ProfileID: profileID,
				Filter:    filter,
				Status:    controld.IntBool(enabled),
			}
			ran, err := ax.Guard(cmd.Context(), func(ctx context.Context) error {
				_, updateErr := client.UpdateProfileFilter(ctx, params)
				return updateErr
			})
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			return ax.WriteJSON(cmd.OutOrStdout(),
				ax.NewEnvelope(cmd.Context(), filterUpdatePayload{ProfileID: profileID, Filter: filter, Updated: ran}))
		},
	}
	cmd.Flags().StringVar(&profileID, "profile-id", "", "profile PK (required)")
	cmd.Flags().StringVar(&filter, "filter", "", "filter PK, e.g. ads (required)")
	cmd.Flags().BoolVar(&enabled, "enabled", true, "enable (true) or disable (false) the filter")
	return cmd
}
```

- [ ] **Step 4: Mount under `profiles` in `root.go`**

`root.go`'s `newProfilesCommand` (Task 5) gains one more line — since `newProfilesFiltersCommand` lives in a different file but the same `main` package, add to `newProfilesCommand` in `profiles.go`:

```go
	cmd.AddCommand(newProfilesFiltersCommand(factory))
```

(This is a one-line edit to `profiles.go`, not `root.go` — `root.go` itself doesn't change for this task.)

- [ ] **Step 5: Run tests, then commit**

Run: `go build ./... && go test ./... -v`
Expected: PASS.

```bash
git add profiles_filters.go profiles_filters_test.go profiles.go internal/controld/client.go
git commit -m "feat: add profile filters list/update commands"
```

---

## Task 7: Profile services

**Files:**
- Create: `profiles_services.go`
- Modify: `internal/controld/client.go`
- Modify: `profiles.go` (mount)
- Test: `profiles_services_test.go`

**Interfaces:**
- Consumes: same as Task 6.
- Produces: `newProfilesServicesCommand(factory clientFactory) *cobra.Command`.

- [ ] **Step 1: Add re-exports**

```go
type ProfileService = controld.ProfileService
type ListProfileServicesParams = controld.ListProfileServicesParams
type UpdateProfileServiceParams = controld.UpdateProfileServiceParams
type Action = controld.Action
```

- [ ] **Step 2: Write the failing test**

```go
// profiles_services_test.go
package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProfilesServicesList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/profiles/p1/services" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true, "body": {"services": [{"PK": "netflix", "name": "Netflix", "category": "streaming", "unlock_location": "", "action": {"do": 0, "status": 1}}]}}`))
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.AddCommand(newProfilesServicesCommand(testFactory(t, server)))
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetArgs([]string{"services", "list", "--profile-id=p1", "--format=json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !bytes.Contains(stdout.Bytes(), []byte("Netflix")) {
		t.Fatalf("expected service name in output: %s", stdout.String())
	}
}
```

- [ ] **Step 3: Write `profiles_services.go`**

```go
// profiles_services.go
package main

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/rshade/ax-go"

	"github.com/rshade/controld-go-mcp/internal/controld"
)

type servicesListPayload struct {
	Services []controld.ProfileService `json:"services"`
}

func newProfilesServicesCommand(factory clientFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "services",
		Short: "Manage a profile's service (app/site) rules",
	}
	cmd.AddCommand(newProfilesServicesListCommand(factory))
	cmd.AddCommand(newProfilesServicesUpdateCommand(factory))
	return cmd
}

func newProfilesServicesListCommand(factory clientFactory) *cobra.Command {
	var profileID string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List a profile's services",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if profileID == "" {
				return ax.NewError(cmd.Context(), "validation_error", "--profile-id is required",
					ax.WithErrorExitCode(ax.ExitValidation))
			}
			client, err := factory(cmd)
			if err != nil {
				return err
			}
			services, err := client.ListProfileServices(cmd.Context(), controld.ListProfileServicesParams{ProfileID: profileID})
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			return ax.WriteJSON(cmd.OutOrStdout(), ax.NewEnvelope(cmd.Context(), servicesListPayload{Services: services}))
		},
	}
	cmd.Flags().StringVar(&profileID, "profile-id", "", "profile PK (required)")
	return cmd
}

type serviceUpdatePayload struct {
	ProfileID string `json:"profile_id"`
	Service   string `json:"service"`
	Updated   bool   `json:"updated"`
}

func newProfilesServicesUpdateCommand(factory clientFactory) *cobra.Command {
	var profileID, service string
	var do int
	var enabled bool

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Set the block/bypass/spoof/redirect action for a service on a profile",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if profileID == "" || service == "" {
				return ax.NewError(cmd.Context(), "validation_error", "--profile-id and --service are required",
					ax.WithErrorExitCode(ax.ExitValidation))
			}
			client, err := factory(cmd)
			if err != nil {
				return err
			}

			params := controld.UpdateProfileServiceParams{
				ProfileID: profileID,
				Service:   service,
				Do:        controld.DoType(do),
				Status:    controld.IntBool(enabled),
			}
			ran, err := ax.Guard(cmd.Context(), func(ctx context.Context) error {
				_, updateErr := client.UpdateProfileService(ctx, params)
				return updateErr
			})
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			return ax.WriteJSON(cmd.OutOrStdout(),
				ax.NewEnvelope(cmd.Context(), serviceUpdatePayload{ProfileID: profileID, Service: service, Updated: ran}))
		},
	}
	cmd.Flags().StringVar(&profileID, "profile-id", "", "profile PK (required)")
	cmd.Flags().StringVar(&service, "service", "", "service PK, e.g. netflix (required)")
	cmd.Flags().IntVar(&do, "do", int(controld.Block), "action: 0=block, 1=bypass, 2=spoof, 3=redirect")
	cmd.Flags().BoolVar(&enabled, "enabled", true, "enable (true) or disable (false) the rule")
	return cmd
}
```

- [ ] **Step 4: Mount in `profiles.go`**

Add `cmd.AddCommand(newProfilesServicesCommand(factory))` to `newProfilesCommand`.

- [ ] **Step 5: Run tests, then commit**

Run: `go build ./... && go test ./... -v`

```bash
git add profiles_services.go profiles_services_test.go profiles.go internal/controld/client.go
git commit -m "feat: add profile services list/update commands"
```

---

## Task 8: Profile custom rules

**Files:**
- Create: `profiles_rules.go`
- Modify: `internal/controld/client.go`
- Modify: `profiles.go` (mount)
- Test: `profiles_rules_test.go`

**Interfaces:**
- Consumes: same as Task 7.
- Produces: `newProfilesRulesCommand(factory clientFactory) *cobra.Command`.

**v1 field scope:** `--folder-id` is required on `list` because `controld-go`'s `ListProfileCustomRules` hard-validates it (returns a local error otherwise) — run `profiles folders list` (Task 9) first to find a valid folder ID. `create`/`update` take `--hostnames` as a comma-separated list (matching `CreateProfileCustomRuleParams.Hostnames []string`); `--via`/`--via-v6`/`--group` are deferred.

- [ ] **Step 1: Add re-exports**

```go
type Rule = controld.Rule
type CustomRule = controld.CustomRule
type ListProfileCustomRulesParams = controld.ListProfileCustomRulesParams
type CreateProfileCustomRuleParams = controld.CreateProfileCustomRuleParams
type UpdateProfileCustomRuleParams = controld.UpdateProfileCustomRuleParams
type DeleteProfileCustomRuleParams = controld.DeleteProfileCustomRuleParams
```

- [ ] **Step 2: Write the failing test**

```go
// profiles_rules_test.go
package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProfilesRulesList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/profiles/p1/rules/f1" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true, "body": {"rules": [{"PK": "example.com", "order": 0, "group": 0, "action": {"do": 0, "status": 1}}]}}`))
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.AddCommand(newProfilesRulesCommand(testFactory(t, server)))
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetArgs([]string{"rules", "list", "--profile-id=p1", "--folder-id=f1", "--format=json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !bytes.Contains(stdout.Bytes(), []byte("example.com")) {
		t.Fatalf("expected rule PK in output: %s", stdout.String())
	}
}

func TestProfilesRulesCreateRequiresYesOrDryRun(t *testing.T) {
	// create is not delete, so it does not require --yes; this test instead
	// confirms --dry-run skips the real POST entirely.
	called := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true, "body": {"rules": []}}`))
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.AddCommand(newProfilesRulesCommand(testFactory(t, server)))
	root.SetArgs([]string{"rules", "create", "--profile-id=p1", "--hostnames=example.com", "--dry-run", "--format=json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if called {
		t.Fatal("expected the real API call to be skipped under --dry-run")
	}
}
```

- [ ] **Step 3: Write `profiles_rules.go`**

```go
// profiles_rules.go
package main

import (
	"context"
	"strings"

	"github.com/spf13/cobra"

	"github.com/rshade/ax-go"

	"github.com/rshade/controld-go-mcp/internal/controld"
)

type rulesListPayload struct {
	Rules []controld.Rule `json:"rules"`
}

type customRulesPayload struct {
	Rules []controld.CustomRule `json:"rules"`
}

type ruleDeletePayload struct {
	ProfileID string `json:"profile_id"`
	Hostname  string `json:"hostname"`
	Deleted   bool   `json:"deleted"`
}

func newProfilesRulesCommand(factory clientFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rules",
		Short: "Manage a profile's custom domain rules",
	}
	cmd.AddCommand(newProfilesRulesListCommand(factory))
	cmd.AddCommand(newProfilesRulesCreateCommand(factory))
	cmd.AddCommand(newProfilesRulesUpdateCommand(factory))
	cmd.AddCommand(newProfilesRulesDeleteCommand(factory))
	return cmd
}

func newProfilesRulesListCommand(factory clientFactory) *cobra.Command {
	var profileID, folderID string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List custom rules in a rule folder",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if profileID == "" || folderID == "" {
				return ax.NewError(cmd.Context(), "validation_error", "--profile-id and --folder-id are required",
					ax.WithActionableFix("run 'profiles folders list --profile-id=<id>' to find a folder ID"),
					ax.WithErrorExitCode(ax.ExitValidation))
			}
			client, err := factory(cmd)
			if err != nil {
				return err
			}
			rules, err := client.ListProfileCustomRules(cmd.Context(), controld.ListProfileCustomRulesParams{
				ProfileID: profileID,
				FolderID:  folderID,
			})
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			return ax.WriteJSON(cmd.OutOrStdout(), ax.NewEnvelope(cmd.Context(), rulesListPayload{Rules: rules}))
		},
	}
	cmd.Flags().StringVar(&profileID, "profile-id", "", "profile PK (required)")
	cmd.Flags().StringVar(&folderID, "folder-id", "", "rule folder ID (required)")
	return cmd
}

func newProfilesRulesCreateCommand(factory clientFactory) *cobra.Command {
	var profileID, hostnames string
	var do int
	var enabled bool

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create custom rules for one or more hostnames",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if profileID == "" || hostnames == "" {
				return ax.NewError(cmd.Context(), "validation_error", "--profile-id and --hostnames are required",
					ax.WithErrorExitCode(ax.ExitValidation))
			}
			client, err := factory(cmd)
			if err != nil {
				return err
			}

			params := controld.CreateProfileCustomRuleParams{
				ProfileID: profileID,
				Do:        controld.DoType(do),
				Status:    controld.IntBool(enabled),
				Hostnames: strings.Split(hostnames, ","),
			}

			var result []controld.CustomRule
			ran, err := ax.Guard(cmd.Context(), func(ctx context.Context) error {
				var createErr error
				result, createErr = client.CreateProfileCustomRule(ctx, params)
				return createErr
			})
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			if !ran {
				result = nil
			}
			return ax.WriteJSON(cmd.OutOrStdout(), ax.NewEnvelope(cmd.Context(), customRulesPayload{Rules: result}))
		},
	}
	cmd.Flags().StringVar(&profileID, "profile-id", "", "profile PK (required)")
	cmd.Flags().StringVar(&hostnames, "hostnames", "", "comma-separated hostnames (required)")
	cmd.Flags().IntVar(&do, "do", int(controld.Block), "action: 0=block, 1=bypass, 2=spoof, 3=redirect")
	cmd.Flags().BoolVar(&enabled, "enabled", true, "enable (true) or disable (false) the rule")
	return cmd
}

func newProfilesRulesUpdateCommand(factory clientFactory) *cobra.Command {
	var profileID, hostnames string
	var do int
	var enabled bool

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update custom rules for one or more hostnames",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if profileID == "" || hostnames == "" {
				return ax.NewError(cmd.Context(), "validation_error", "--profile-id and --hostnames are required",
					ax.WithErrorExitCode(ax.ExitValidation))
			}
			client, err := factory(cmd)
			if err != nil {
				return err
			}

			params := controld.UpdateProfileCustomRuleParams{
				ProfileID: profileID,
				Do:        controld.DoType(do),
				Status:    controld.IntBool(enabled),
				Hostnames: strings.Split(hostnames, ","),
			}

			var result []controld.CustomRule
			ran, err := ax.Guard(cmd.Context(), func(ctx context.Context) error {
				var updateErr error
				result, updateErr = client.UpdateProfileCustomRule(ctx, params)
				return updateErr
			})
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			if !ran {
				result = nil
			}
			return ax.WriteJSON(cmd.OutOrStdout(), ax.NewEnvelope(cmd.Context(), customRulesPayload{Rules: result}))
		},
	}
	cmd.Flags().StringVar(&profileID, "profile-id", "", "profile PK (required)")
	cmd.Flags().StringVar(&hostnames, "hostnames", "", "comma-separated hostnames (required)")
	cmd.Flags().IntVar(&do, "do", int(controld.Block), "action: 0=block, 1=bypass, 2=spoof, 3=redirect")
	cmd.Flags().BoolVar(&enabled, "enabled", true, "enable (true) or disable (false) the rule")
	return cmd
}

func newProfilesRulesDeleteCommand(factory clientFactory) *cobra.Command {
	var profileID, hostname string

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete the custom rule for a hostname",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if profileID == "" || hostname == "" {
				return ax.NewError(cmd.Context(), "validation_error", "--profile-id and --hostname are required",
					ax.WithErrorExitCode(ax.ExitValidation))
			}

			subject := "delete custom rule for " + hostname
			outcome, err := ax.Confirm(cmd.Context(), subject)
			if err != nil {
				return err
			}
			if outcome == ax.ConfirmationPromptRequired {
				approved, promptErr := promptForConfirmation(cmd, subject)
				if promptErr != nil {
					return promptErr
				}
				if !approved {
					return ax.WriteJSON(cmd.OutOrStdout(),
						ax.NewEnvelope(cmd.Context(), ruleDeletePayload{ProfileID: profileID, Hostname: hostname, Deleted: false}))
				}
			}

			client, err := factory(cmd)
			if err != nil {
				return err
			}

			ran, err := ax.Guard(cmd.Context(), func(ctx context.Context) error {
				_, deleteErr := client.DeleteProfileCustomRule(ctx, controld.DeleteProfileCustomRuleParams{
					ProfileID: profileID,
					Hostname:  hostname,
				})
				return deleteErr
			})
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			return ax.WriteJSON(cmd.OutOrStdout(),
				ax.NewEnvelope(cmd.Context(), ruleDeletePayload{ProfileID: profileID, Hostname: hostname, Deleted: ran}))
		},
	}
	cmd.Flags().StringVar(&profileID, "profile-id", "", "profile PK (required)")
	cmd.Flags().StringVar(&hostname, "hostname", "", "hostname to remove the rule for (required)")
	return cmd
}
```

- [ ] **Step 4: Mount in `profiles.go`**

Add `cmd.AddCommand(newProfilesRulesCommand(factory))` to `newProfilesCommand`.

- [ ] **Step 5: Run tests, then commit**

Run: `go build ./... && go test ./... -v`

```bash
git add profiles_rules.go profiles_rules_test.go profiles.go internal/controld/client.go
git commit -m "feat: add profile custom rules list/create/update/delete commands"
```

---

## Task 9: Profile rule folders

**Files:**
- Create: `profiles_folders.go`
- Modify: `internal/controld/client.go`
- Modify: `profiles.go` (mount)
- Test: `profiles_folders_test.go`

**Interfaces:**
- Consumes: same as Task 8.
- Produces: `newProfilesFoldersCommand(factory clientFactory) *cobra.Command`.

**v1 field scope:** `create`/`update` expose `--do` and `--status`; `--via` is deferred.

- [ ] **Step 1: Add re-exports**

```go
type Group = controld.Group
type GroupAction = controld.GroupAction
type ListProfileRuleFoldersParams = controld.ListProfileRuleFoldersParams
type CreateProfileRuleFolderParams = controld.CreateProfileRuleFolderParams
type UpdateProfileRuleFolderParams = controld.UpdateProfileRuleFolderParams
type DeleteProfileRuleFolderParams = controld.DeleteProfileRuleFolderParams
```

- [ ] **Step 2: Write the failing test**

```go
// profiles_folders_test.go
package main

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestProfilesFoldersList(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/profiles/p1/groups" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success": true, "body": {"groups": [{"PK": 0, "group": "Default", "action": {"status": 1}, "count": 3}]}}`))
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.AddCommand(newProfilesFoldersCommand(testFactory(t, server)))
	var stdout bytes.Buffer
	root.SetOut(&stdout)
	root.SetArgs([]string{"folders", "list", "--profile-id=p1", "--format=json"})
	if err := root.Execute(); err != nil {
		t.Fatalf("execute: %v", err)
	}
	if !bytes.Contains(stdout.Bytes(), []byte("Default")) {
		t.Fatalf("expected folder name in output: %s", stdout.String())
	}
}

func TestProfilesFoldersDeleteRequiresYes(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("the real API should never be called without --yes in machine mode")
	}))
	defer server.Close()

	root := newRootCommand(testFactory(t, server))
	root.AddCommand(newProfilesFoldersCommand(testFactory(t, server)))
	root.SetArgs([]string{"folders", "delete", "--profile-id=p1", "--folder-id=f1", "--format=json"})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected a confirmation_required error without --yes")
	}
}
```

- [ ] **Step 3: Write `profiles_folders.go`**

```go
// profiles_folders.go
package main

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/rshade/ax-go"

	"github.com/rshade/controld-go-mcp/internal/controld"
)

type foldersListPayload struct {
	Groups []controld.Group `json:"groups"`
}

type folderDeletePayload struct {
	ProfileID string `json:"profile_id"`
	FolderID  string `json:"folder_id"`
	Deleted   bool   `json:"deleted"`
}

func newProfilesFoldersCommand(factory clientFactory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "folders",
		Short: "Manage a profile's custom-rule folders",
	}
	cmd.AddCommand(newProfilesFoldersListCommand(factory))
	cmd.AddCommand(newProfilesFoldersCreateCommand(factory))
	cmd.AddCommand(newProfilesFoldersUpdateCommand(factory))
	cmd.AddCommand(newProfilesFoldersDeleteCommand(factory))
	return cmd
}

func newProfilesFoldersListCommand(factory clientFactory) *cobra.Command {
	var profileID string

	cmd := &cobra.Command{
		Use:   "list",
		Short: "List a profile's rule folders",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if profileID == "" {
				return ax.NewError(cmd.Context(), "validation_error", "--profile-id is required",
					ax.WithErrorExitCode(ax.ExitValidation))
			}
			client, err := factory(cmd)
			if err != nil {
				return err
			}
			groups, err := client.ListProfileRuleFolders(cmd.Context(), controld.ListProfileRuleFoldersParams{ProfileID: profileID})
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			return ax.WriteJSON(cmd.OutOrStdout(), ax.NewEnvelope(cmd.Context(), foldersListPayload{Groups: groups}))
		},
	}
	cmd.Flags().StringVar(&profileID, "profile-id", "", "profile PK (required)")
	return cmd
}

func newProfilesFoldersCreateCommand(factory clientFactory) *cobra.Command {
	var profileID, name string
	var do int
	var enabled bool

	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a rule folder",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if profileID == "" || name == "" {
				return ax.NewError(cmd.Context(), "validation_error", "--profile-id and --name are required",
					ax.WithErrorExitCode(ax.ExitValidation))
			}
			client, err := factory(cmd)
			if err != nil {
				return err
			}

			doVal := controld.DoType(do)
			statusVal := controld.IntBool(enabled)
			params := controld.CreateProfileRuleFolderParams{
				ProfileID: profileID,
				Name:      name,
				Do:        &doVal,
				Status:    &statusVal,
			}

			var result []controld.Group
			ran, err := ax.Guard(cmd.Context(), func(ctx context.Context) error {
				var createErr error
				result, createErr = client.CreateProfileRuleFolder(ctx, params)
				return createErr
			})
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			if !ran {
				result = nil
			}
			return ax.WriteJSON(cmd.OutOrStdout(), ax.NewEnvelope(cmd.Context(), foldersListPayload{Groups: result}))
		},
	}
	cmd.Flags().StringVar(&profileID, "profile-id", "", "profile PK (required)")
	cmd.Flags().StringVar(&name, "name", "", "folder name (required)")
	cmd.Flags().IntVar(&do, "do", int(controld.Block), "default action for rules in this folder: 0=block, 1=bypass, 2=spoof, 3=redirect")
	cmd.Flags().BoolVar(&enabled, "enabled", true, "enable (true) or disable (false) the folder")
	return cmd
}

func newProfilesFoldersUpdateCommand(factory clientFactory) *cobra.Command {
	var profileID, folderID string
	var do int
	var enabled bool

	cmd := &cobra.Command{
		Use:   "update",
		Short: "Update a rule folder",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if profileID == "" || folderID == "" {
				return ax.NewError(cmd.Context(), "validation_error", "--profile-id and --folder-id are required",
					ax.WithErrorExitCode(ax.ExitValidation))
			}
			client, err := factory(cmd)
			if err != nil {
				return err
			}

			doVal := controld.DoType(do)
			statusVal := controld.IntBool(enabled)
			params := controld.UpdateProfileRuleFolderParams{
				ProfileID: profileID,
				FolderID:  folderID,
				Do:        &doVal,
				Status:    &statusVal,
			}

			var result []controld.Group
			ran, err := ax.Guard(cmd.Context(), func(ctx context.Context) error {
				var updateErr error
				result, updateErr = client.UpdateProfileRuleFolder(ctx, params)
				return updateErr
			})
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			if !ran {
				result = nil
			}
			return ax.WriteJSON(cmd.OutOrStdout(), ax.NewEnvelope(cmd.Context(), foldersListPayload{Groups: result}))
		},
	}
	cmd.Flags().StringVar(&profileID, "profile-id", "", "profile PK (required)")
	cmd.Flags().StringVar(&folderID, "folder-id", "", "folder ID to update (required)")
	cmd.Flags().IntVar(&do, "do", int(controld.Block), "default action for rules in this folder: 0=block, 1=bypass, 2=spoof, 3=redirect")
	cmd.Flags().BoolVar(&enabled, "enabled", true, "enable (true) or disable (false) the folder")
	return cmd
}

func newProfilesFoldersDeleteCommand(factory clientFactory) *cobra.Command {
	var profileID, folderID string

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete a rule folder",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if profileID == "" || folderID == "" {
				return ax.NewError(cmd.Context(), "validation_error", "--profile-id and --folder-id are required",
					ax.WithErrorExitCode(ax.ExitValidation))
			}

			subject := "delete rule folder " + folderID
			outcome, err := ax.Confirm(cmd.Context(), subject)
			if err != nil {
				return err
			}
			if outcome == ax.ConfirmationPromptRequired {
				approved, promptErr := promptForConfirmation(cmd, subject)
				if promptErr != nil {
					return promptErr
				}
				if !approved {
					return ax.WriteJSON(cmd.OutOrStdout(),
						ax.NewEnvelope(cmd.Context(), folderDeletePayload{ProfileID: profileID, FolderID: folderID, Deleted: false}))
				}
			}

			client, err := factory(cmd)
			if err != nil {
				return err
			}

			ran, err := ax.Guard(cmd.Context(), func(ctx context.Context) error {
				_, deleteErr := client.DeleteProfileRuleFolder(ctx, controld.DeleteProfileRuleFolderParams{
					ProfileID: profileID,
					FolderID:  folderID,
				})
				return deleteErr
			})
			if err != nil {
				return controld.MapError(cmd.Context(), err)
			}
			return ax.WriteJSON(cmd.OutOrStdout(),
				ax.NewEnvelope(cmd.Context(), folderDeletePayload{ProfileID: profileID, FolderID: folderID, Deleted: ran}))
		},
	}
	cmd.Flags().StringVar(&profileID, "profile-id", "", "profile PK (required)")
	cmd.Flags().StringVar(&folderID, "folder-id", "", "folder ID to delete (required)")
	return cmd
}
```

- [ ] **Step 4: Mount in `profiles.go`**

Add `cmd.AddCommand(newProfilesFoldersCommand(factory))` to `newProfilesCommand`.

- [ ] **Step 5: Run tests, then commit**

Run: `go build ./... && go test ./... -v`

```bash
git add profiles_folders.go profiles_folders_test.go profiles.go internal/controld/client.go
git commit -m "feat: add profile rule folders list/create/update/delete commands"
```

---

## Task 10: End-to-end verification, lint, and README

**Files:**
- Modify: `README.md` (create if it doesn't already have usage content)
- No new source files — this task is verification, not new behavior.

**Interfaces:** none new — this task only exercises what Tasks 1–9 already built.

- [ ] **Step 1: Full build and test sweep**

Run:
```bash
go build -o bin/controldctl .
go vet ./...
gofmt -l .
go test ./... -v
```
Expected: clean build, no `go vet` findings, no `gofmt -l` output (meaning nothing is misformatted), all tests pass.

- [ ] **Step 2: golangci-lint, if available**

Run: `golangci-lint run ./...` (use `~/.local/bin/golangci-lint-versions/golangci-lint` if that's the pinned version for this workspace; otherwise whatever `golangci-lint` resolves to on `$PATH`).
Expected: no findings. Fix anything it flags before proceeding — do not suppress with `//nolint` unless the finding is a genuine false positive, and say why in a comment if you do.

- [ ] **Step 3: Walk the full command tree manually**

```bash
export CONTROLD_API_TOKEN=<a real ControlD API token, Read+Write scope>
./bin/controldctl __schema | jq '.commands | keys'
./bin/controldctl devices list --format=json
./bin/controldctl profiles list --format=json
./bin/controldctl profiles options list --format=json
./bin/controldctl profiles filters list --profile-id=<real-profile-id> --format=json
./bin/controldctl profiles services list --profile-id=<real-profile-id> --format=json
./bin/controldctl profiles folders list --profile-id=<real-profile-id> --format=json
./bin/controldctl profiles rules list --profile-id=<real-profile-id> --folder-id=<real-folder-id> --format=json
./bin/controldctl mcp-server --help
./bin/controldctl --mcp --help
```

Expected: every command runs against the real ControlD API and returns a JSON envelope (or a correctly-mapped `ax.Error` if, say, the token lacks write scope). This step needs a real account and API token — if one isn't available, skip the real-network parts and rely on Tasks 1–9's `httptest`-backed tests as the completion bar; note in the commit message that live verification is still pending.

- [ ] **Step 4: Write `README.md`**

```markdown
# controldctl

A CLI and MCP server for the [ControlD](https://controld.com) DNS filtering API, built on [ax-go](https://github.com/rshade/ax-go) and [baptistecdr/controld-go](https://github.com/baptistecdr/controld-go).

See [`docs/superpowers/specs/2026-08-28-controldctl-cli-mcp-design.md`](docs/superpowers/specs/2026-08-28-controldctl-cli-mcp-design.md) for the full design.

## Install

\`\`\`bash
go build -o bin/controldctl .
\`\`\`

## Authenticate

Set `CONTROLD_API_TOKEN`, or pass `--api-token`, or point `--config` at a Hujson file with an `api_token` field.

## Usage

\`\`\`bash
controldctl devices list --format=json
controldctl profiles list --format=json
controldctl profiles filters list --profile-id=<id>
controldctl profiles rules create --profile-id=<id> --hostnames=ads.example.com --do=0
controldctl mcp-server                 # run as an MCP server over stdio
controldctl --mcp                      # same thing, shorter
\`\`\`

Every mutating command supports `--dry-run`; every `delete` command requires `--yes` (or an interactive confirmation prompt in human mode).
\`\`\`
```

Run `markdownlint README.md` if the `markdownlint` skill/command is available in this environment, and fix any findings.

- [ ] **Step 5: Commit**

```bash
git add README.md
git commit -m "docs: add controldctl README with usage examples"
```

---

## Post-plan follow-ups (not part of this plan)

- Users/account info, Billing, Network Stats, Access/IP logs, org impersonation, and mass provisioning — each needs its own scoping pass per the design spec.
- Optional flags deferred per resource (DDNS/legacy-IPv4 on devices, `disable_ttl`/`lock_status` on profiles, `via`/`group`/`order` on rules and folders) — extend the existing flag-registration pattern in the relevant command file; no new architecture needed.
- A real end-to-end smoke test tier gated on `CONTROLD_API_TOKEN` being present locally (never in CI), per the design spec's testing section.
