# Contributing to controldctl

controldctl is a CLI and MCP server for the [ControlD](https://controld.com)
DNS filtering API. It's built on [ax-go](https://github.com/rshade/ax-go) for
the CLI/agent runtime and
[baptistecdr/controld-go](https://github.com/baptistecdr/controld-go) as the
underlying API client. This guide covers local development, the pattern for
adding a new resource command, testing conventions, and commit expectations.

## Development setup

Requires Go 1.26.7 or newer (see `go.mod`). All common tasks are `Makefile`
targets:

```bash
make build   # go build -o bin/controldctl ./cmd/controldctl
make test    # go test ./...
make lint    # go vet ./... && gofmt -l .
make fmt     # gofmt -w .
make tidy    # go mod tidy
make race    # go test -race ./...
make ci      # runs build, lint, and race — a subset of what CI checks
```

Run `make fmt` before `make lint` — `gofmt -l .` only _lists_ files that
aren't formatted, it doesn't fix them, and (tracked as roadmap #2) `make
lint` doesn't yet fail on its own when files are listed, so treat any
output as a failure regardless of exit code. `make lint` must report no
output (both `go vet` and `gofmt -l`), and `make race` must pass to catch
data races before you open a PR. Alternatively, run `make ci` to execute
build, lint, and the race-tested suite together. CI runs these same checks
automatically on every push to `main` and pull request via
`.github/workflows/ci.yml`, plus `golangci-lint`, `govulncheck`, and
`actionlint` against the workflow itself — `make ci` is a useful local
subset, not full parity, aside from the gofmt-enforcement gap above.

## Adding a new resource command

Every resource command in `cmd/controldctl` follows the same shape. The
`profiles services` command
(`cmd/controldctl/profiles_services.go`,
`cmd/controldctl/profiles_services_test.go`) is the simplest complete
example — a `list` and an `update` command wrapping
`baptistecdr/controld-go`'s `ProfileService` type. Use it as your template.

### 1. Re-export the vendor types you need in `internal/controld/client.go`

`package main` never imports `github.com/baptistecdr/controld-go` directly.
Everything it needs comes through `internal/controld`, via type aliases:

```go
// Service-related re-exports for the profiles services command.
type ProfileService = controld.ProfileService
type ListProfileServicesParams = controld.ListProfileServicesParams
type UpdateProfileServiceParams = controld.UpdateProfileServiceParams
type Action = controld.Action
```

Only re-export what the new command actually uses — don't add speculative
aliases for types nothing calls yet. Note that a type alias does not carry
forward the vendor package's constants; if you need one (like
`controld.DesktopLinux`), re-export it explicitly as its own `const`.

### 2. Decide whether you need a local payload wrapper

Check the vendor struct's actual JSON tags — don't assume. Some
`controld-go` types tag their primary key field uppercase:

```go
// baptistecdr/controld-go's services.go
type Service struct {
    PK             string  `json:"PK"`
    Name           string  `json:"name"`
    ...
}
```

If any field uses an uppercase (or otherwise non-idiomatic) tag, write a
local payload struct with corrected lowercase tags and a
`toXPayload`/`toXPayloads` conversion function that copies fields by name
(not by embedding, since embedding would inherit the vendor tags):

```go
type servicePayload struct {
    PK             string          `json:"pk"`
    Name           string          `json:"name"`
    Category       string          `json:"category"`
    UnlockLocation string          `json:"unlock_location"`
    Locations      []string        `json:"locations,omitempty"`
    Action         controld.Action `json:"action"`
    Warning        *string         `json:"warning,omitempty"`
}

func toServicePayload(s controld.ProfileService) servicePayload {
    return servicePayload{
        PK:             s.PK,
        Name:           s.Name,
        Category:       s.Category,
        UnlockLocation: s.UnlockLocation,
        Locations:      s.Locations,
        Action:         s.Action,
        Warning:        s.Warning,
    }
}

func toServicePayloads(services []controld.ProfileService) []servicePayload {
    payloads := make([]servicePayload, 0, len(services))
    for _, s := range services {
        payloads = append(payloads, toServicePayload(s))
    }
    return payloads
}
```

If the vendor type already uses lowercase tags throughout (no wrapper
needed), pass it straight through as the payload. `devices types` does
this with `controld.DeviceTypes`, whose fields (`OS`, `Browser`, `TV`,
`Router`) are all tagged lowercase already:

```go
types, err := client.ListDeviceType(cmd.Context())
if err != nil {
    return controld.MapError(cmd.Context(), err)
}
return ax.WriteJSON(cmd.OutOrStdout(), ax.NewEnvelope(cmd.Context(), types))
```

### 3. Tag genuinely non-deterministic fields

If a field is a server-generated identifier or timestamp that changes on
`create` (not a stable catalog ID like a service's `netflix` PK), tag it
`ax:"nondeterministic"` on the payload wrapper:

```go
type devicePayload struct {
    PK          string `json:"pk"           ax:"nondeterministic"`
    DeviceID    string `json:"device_id"    ax:"nondeterministic"`
    Name        string `json:"name"`
    Status      int    `json:"status"`
    ProfilePK   string `json:"profile_pk"`
    ProfileName string `json:"profile_name"`
    CreatedAt   int64  `json:"created_at"   ax:"nondeterministic"`
}
```

Then call `ax.WithNonDeterministicFields[T](cmd)` — with the payload type as
the generic parameter — on every `list`, `create`, and `update` command that
returns the payload, so `__schema` output correctly marks those fields:

```go
ax.WithNonDeterministicFields[devicesListPayload](cmd)
```

Call this even when the payload has no tagged fields (e.g. a catalog
resource like `profiles services list`, where the PK is stable) — it's what
registers the command's response schema for `__schema`, not just the
non-deterministic markers.

### 4. Write the command constructor(s)

Every command follows the same skeleton: build a `*cobra.Command`, validate
required flags locally, get a client via the injected `clientFactory`, call
the vendor method, map any error, write the envelope.

```go
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
            payload := servicesListPayload{Services: toServicePayloads(services)}
            return ax.WriteJSON(cmd.OutOrStdout(), ax.NewEnvelope(cmd.Context(), payload))
        },
    }
    cmd.Flags().StringVar(&profileID, "profile-id", "", "profile PK (required)")
    ax.WithNonDeterministicFields[servicesListPayload](cmd)
    return cmd
}
```

Points to note:

- `clientFactory` (defined in `cmd/controldctl/root.go`) is
  `func(cmd *cobra.Command) (*controld.API, error)` — every command
  constructor takes it as a parameter and calls it inside `RunE`, not at
  construction time. This is what lets tests inject an `httptest.Server`
  client instead of a real one.
- Required-flag validation happens first, before the client is constructed,
  and returns `ax.NewError(ctx, "validation_error", "...", ax.WithErrorExitCode(ax.ExitValidation))`.
- Any error from the vendor client is routed through `controld.MapError`
  before being returned — never returned raw. `MapError` (in
  `internal/controld/errors.go`) translates vendor error types (e.g.
  `*controld.AuthorizationError`, `*controld.NotFoundError`) into `ax.Error`
  values with the right exit code and an actionable fix.
- The response is always written with
  `ax.WriteJSON(cmd.OutOrStdout(), ax.NewEnvelope(cmd.Context(), payload))`.

### 5. Wrap mutating calls in `ax.Guard`

Any command with a side effect (`create`, `update`, `delete`) wraps the
vendor call in `ax.Guard` so `--dry-run` skips it while the response
envelope is still written:

```go
ran, err := ax.Guard(cmd.Context(), func(ctx context.Context) error {
    _, updateErr := client.UpdateProfileService(ctx, params)
    return updateErr
})
if err != nil {
    return controld.MapError(cmd.Context(), err)
}
return ax.WriteJSON(cmd.OutOrStdout(),
    ax.NewEnvelope(cmd.Context(), serviceUpdatePayload{ProfileID: profileID, Service: service, Updated: ran}))
```

`ran` reports whether the effect actually executed, which is why the
`...Updated`/`...Deleted` field in the response payload is set from it
rather than hardcoded `true`.

This value-type shape is correct when the payload is built entirely from
flags already in scope — every `delete` and `profiles
options/filters/services update`. It is the wrong choice for a `create` or
`update` that echoes back state the API returned, because under `--dry-run`
no API call happens and there is no state to report; reusing the value-type
pattern there would silently produce a zero-valued struct instead of an
honest empty response. For those commands, hold the payload as a pointer and
only populate it when `ran` is `true`, as `devices create` does:

```go
var result controld.Device
ran, err := ax.Guard(cmd.Context(), func(ctx context.Context) error {
    var createErr error
    result, createErr = client.CreateDevice(ctx, params)
    return createErr
})
if err != nil {
    return controld.MapError(cmd.Context(), err)
}
var payload *devicePayload
if ran {
    device := toDevicePayload(result)
    payload = &device
}
return ax.WriteJSON(cmd.OutOrStdout(), ax.NewEnvelope(cmd.Context(), payload))
```

`ax.NewEnvelope` marshals a `nil` pointer to `data: null`, which is the
contract documented for `--dry-run` under **Global flags** in
`docs/commands.md` — see that section for the full breakdown of which
commands use which shape.

For `delete` specifically, gate on `ax.Confirm` (and, in human mode, the
`promptForConfirmation` helper from `root.go`) _before_ reaching
`ax.Guard`, so a user is never prompted for a call that `--dry-run` would
skip anyway:

```go
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
```

In machine mode (`--format=json`) without `--yes`, `ax.Confirm` returns
`ConfirmationBlocked` and a `confirmation_required` validation error — the
command never reaches the client or the guard. In human mode without
`--yes`, it returns `ConfirmationPromptRequired` and the command owns
prompting the user itself.

Every mutating command also needs a case in the golden table
(`cmd/controldctl/golden_test.go`) with its `dryRunShape` declared:
`shapeNull` when the payload echoes API-returned state, `shapeIdentity` when
it's built from the caller's flags, and `shapeNone` for read-only commands.
The zero value, `shapeUnset`, deliberately fails the test with "dry-run shape
not declared" — the shape must be declared explicitly so the dry-run contract
can't silently regress when a new command lands without coverage. Regenerate
fixtures for a new or changed case with:

```bash
go test ./cmd/controldctl -run TestGolden -update
```

### 6. Mount the command in its parent

Add the new subcommand to its parent's constructor:

```go
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
    cmd.AddCommand(newProfilesFiltersCommand(factory))
    cmd.AddCommand(newProfilesServicesCommand(factory))
    cmd.AddCommand(newProfilesRulesCommand(factory))
    cmd.AddCommand(newProfilesFoldersCommand(factory))
    return cmd
}
```

If you're adding an entirely new top-level resource (not a subcommand of
`devices` or `profiles`), mount it in `newRootCommand` in `root.go` instead.

## Testing conventions

Every command test builds the real command tree via `newRootCommand(...)`
and drives it through `ax.Execute`, never a bare `root.Execute()`:

```go
root := newRootCommand(testFactory(t, server))
root.SetArgs([]string{"profiles", "services", "list", "--profile-id=p1", "--format=json"})

var stdout, stderr bytes.Buffer
code := ax.Execute(context.Background(), root,
    ax.WithStdout(&stdout),
    ax.WithStderr(&stderr),
    ax.WithEnv(func(string) string { return "" }),
)
if code != ax.ExitSuccess {
    t.Fatalf("exit code = %d, want %d; stderr=%s", code, ax.ExitSuccess, stderr.String())
}
```

This matters because `--format`, `--dry-run`, and `--yes` are registered as
persistent flags by `ax.Execute` itself, not by `newRootCommand` or any
command constructor. A bare `root.Execute()` call never parses those flags,
so a test written that way would either fail to compile the args it needs
or silently pass for the wrong reason. Always assert on the returned exit
code (`ax.ExitSuccess`, `ax.ExitValidation`, etc.), not just on stdout
content.

HTTP interaction is tested against a real `httptest.Server`, never mocked
at a higher level. The shared `testFactory` helper (defined once, in
`cmd/controldctl/devices_test.go`, reused by every other `_test.go` file in
the package) builds a `clientFactory` pointed at the test server:

```go
func testFactory(t *testing.T, server *httptest.Server) clientFactory {
    t.Helper()
    return func(cmd *cobra.Command) (*controld.API, error) {
        return controld.NewClient(cmd.Context(), "test-token", "", server.URL, func(string) string { return "" })
    }
}
```

It works by passing `server.URL` as the `baseURL` parameter of
`controld.NewClient` — the same override point production code leaves empty
to hit the real ControlD API. A typical test spins up an
`httptest.NewServer` handler that asserts on the request path/method and
returns a fixture JSON body, then runs the command and asserts on the
decoded response envelope:

```go
server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    if r.URL.Path != "/profiles/p1/services" {
        t.Fatalf("unexpected path: %s", r.URL.Path)
    }
    w.Header().Set("Content-Type", "application/json")
    _, _ = w.Write([]byte(`{"success": true, "body": {"services": [...]}}`))
}))
defer server.Close()
```

Cover at minimum: a successful call decoding the expected envelope fields,
and a validation-error case for each required flag being omitted.

## Commit and PR expectations

There's no commitlint configuration in this repo, but the git history
consistently uses conventional-commit-style prefixes:

```text
feat: add profile custom rules list/create/update/delete commands
fix: remap nested Opt/FilterLevel PK casing in filter payloads
docs: add controldctl README with usage examples
refactor: move to standard cmd/controldctl layout
build: pin ax-go to the published v0.5.0 release
chore: rename module to github.com/rshade/controldctl
```

Use `feat:`, `fix:`, `docs:`, `refactor:`, `build:`, `chore:`, or `test:` as
appropriate, with a short imperative summary. Before opening a PR:

- `make ci` passes cleanly (or equivalently, `make build && make lint && make race` all pass).
- New commands have test coverage for both the success path and required-flag
  validation, following the pattern above.
- The `README.md` command tree and any usage examples are updated if the
  command surface changed.
