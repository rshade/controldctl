# Architecture

`controldctl` is a Go CLI for the [ControlD](https://controld.com) DNS
filtering API. The same Cobra command tree that serves the CLI also drives an
MCP server (`controldctl mcp-server`, or the `controldctl --mcp` shorthand),
so every command is simultaneously a terminal-friendly subcommand and a tool
an LLM agent can call directly. This document explains how the codebase is
put together and why it's shaped the way it is.

Two libraries carry most of the weight:

- [`github.com/rshade/ax-go`](https://github.com/rshade/ax-go) — a shared
  framework that gives any Cobra CLI deterministic JSON output envelopes,
  structured errors with exit codes, dry-run/confirmation safety primitives,
  and MCP server generation derived from the same command tree.
- [`github.com/baptistecdr/controld-go`](https://github.com/baptistecdr/controld-go)
  (pinned at v0.0.10) — the community ControlD API client that actually talks
  to `api.controld.com`.

## Package layout

```text
cmd/controldctl/     package main — the entire Cobra command tree
internal/controld/   the only package that imports controld-go directly
```

`cmd/controldctl/` holds every command file (`devices.go`, `profiles.go`,
`profiles_filters.go`, and so on), root wiring (`root.go`), and the process
entry point with its `--mcp` shim (`main.go`). All of it is `package main`.

`internal/controld/` is the sole boundary to the vendor library. Every
command file reaches upstream ControlD types exclusively through this
package's re-exports — no file under `cmd/controldctl/` imports
`github.com/baptistecdr/controld-go` itself. `internal/controld/client.go`
states this explicitly:

```go
// Package controld adapts baptistecdr/controld-go for controldctl. Files in
// this package import "github.com/baptistecdr/controld-go" directly and
// reference it as controld.X: that's legal even though this package is also
// named controld, because a Go file never self-qualifies its own package's
// members with the package name — controld.X here can only resolve to the
// import. Every other package in this module imports only this package
// (internal/controld) and never the upstream library directly.
```

This boundary matters for one practical reason: the vendor dependency is
isolated to a single package. If `baptistecdr/controld-go` needs to be
upgraded across a breaking change, swapped for a different client, or forked
outright, the change is contained to `internal/controld/client.go` and
`internal/controld/errors.go`. The command tree in `cmd/controldctl/` never
has to change just because the vendor library's internals did.

## The type-alias re-export pattern

Most of what `internal/controld/client.go` exports is a zero-cost type
alias, not a wrapper struct:

```go
type Device = controld.Device
type CreateDeviceParams = controld.CreateDeviceParams
type Profile = controld.Profile
type Action = controld.Action
```

An alias (`type X = controld.X`, with the `=`) makes `controld.Device` and
`controld.Profile` in `internal/controld` refer to the exact same type as
`controld.Device` from the vendor package — not a new type with the same
shape. There's no conversion, no allocation, and no field-by-field copy at
the alias site.

This is deliberate for two kinds of types:

- **Request parameter types** (`CreateDeviceParams`, `UpdateProfileParams`,
  `ListProfileFiltersParams`, ...) — these are never written to stdout, so
  there's no JSON-casing concern. Command code needs to construct exactly
  the struct `controld-go`'s API methods expect, so an alias is the only
  sensible choice.
- **Response types whose JSON shape is already correct as-is** for internal
  use — for example, `Action` and `GroupAction` are aliased directly and
  embedded into payload structs unchanged (see the next section), because
  their own JSON tags already match this CLI's house style.

Aliases are the default. A payload wrapper (below) is only introduced when a
vendor type's JSON tags don't match what should actually reach stdout.

## The casing-wrapper convention

Several vendor types use uppercase JSON tags — most commonly `PK`:

```go
// baptistecdr/controld-go
type Device struct {
    PK   string   `json:"PK"`
    Ts   UnixTime `json:"ts"`
    Name string   `json:"name"`
    ...
}
```

`controldctl`'s house style is lowercase, snake_case JSON. Rather than expose
`controld.Device` (and its `"PK"` tag) directly on stdout, any type actually
written to a response envelope gets a local payload struct with corrected
tags. `cmd/controldctl/devices.go` defines `devicePayload` for exactly this
reason:

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

`toDevicePayload` does the field-by-field copy from `controld.Device`,
flattening the nested `Profile` struct into `profile_pk`/`profile_name` and
converting `Ts` (a `UnixTime`) to a plain Unix timestamp along the way.
`cmd/controldctl/profiles_filters.go` shows the same pattern nested one
level deeper: `filterPayload` wraps `controld.Filter` (tag `"PK"`), and its
`Levels` field is itself `[]filterLevelPayload` — a wrapper around
`controld.FilterLevel`, whose own `Opt` field wraps `controld.Opt` (also
tagged `"PK"`) as `[]filterOptPayload`. Every uppercase-tagged field gets
rewrapped all the way down the nesting.

Not every vendor type needs this treatment. Some already use lowercase JSON
tags and are embedded directly into payload structs with no wrapper at all.
`cmd/controldctl/profiles_services.go` embeds `controld.Action` as-is:

```go
type servicePayload struct {
    PK             string          `json:"pk"`
    Name           string          `json:"name"`
    ...
    Action         controld.Action `json:"action"`
    Warning        *string         `json:"warning,omitempty"`
}
```

`Action`'s own fields (`Do`, `Status`, `Via`, `ViaV6`, `Group`, `Order`) are
already `json:"do"`, `json:"status"`, and so on — no wrapper needed.
`CustomRule` (declared upstream as `type CustomRule Action`) and
`GroupAction` are in the same position and are likewise embedded directly, as
`cmd/controldctl/profiles_rules.go` and `cmd/controldctl/profiles_folders.go`
note in comments beside their wrapper types.

The rule for telling which is which is not a guess: check the vendor
struct's actual JSON tags in `controld-go`'s source before deciding. A type
with an uppercase tag anywhere in its field list gets a local wrapper; a type
whose tags are already all-lowercase gets embedded directly.

## Non-determinism tagging

Some response fields are genuinely fresh on every call that produces them —
server-generated identifiers assigned at creation time, like `Device.PK`,
`Device.DeviceID`, `Profile.PK`, and `Group.PK` (rule folders). Others are
stable catalog identifiers that don't vary between calls at all — filter PKs
like `ads`, service PKs like `netflix`, profile option names, and
hostname-keyed custom rules.

Wrapper structs mark the first kind with an `ax:"nondeterministic"` struct
tag, as seen above on `devicePayload.PK`, `.DeviceID`, and `.CreatedAt`.
`profilePayload` and `groupPayload` do the same for their own `PK` fields:

```go
type profilePayload struct {
    PK        string `json:"pk"         ax:"nondeterministic"`
    Name      string `json:"name"`
    UpdatedAt int64  `json:"updated_at" ax:"nondeterministic"`
}
```

Every command whose output includes a tagged type registers those fields
with `ax.WithNonDeterministicFields[T](cmd)`, typically right before
`return cmd` at the end of the constructor function:

```go
ax.WithNonDeterministicFields[devicesListPayload](cmd)
```

This is registered on `list`, `create`, and `update` commands for
resources with server-generated identifiers — the commands whose output can
actually vary run to run. Filters, services, profile options, and custom
rules carry no such tag anywhere in their payload structs, because their
identifiers are caller-known and stable.

The payoff is discoverability without a live call: an agent inspecting
`controldctl __schema` can see in advance which output fields will differ
between two otherwise-identical invocations, without needing to run the
command twice and diff the results to find out empirically.

## Error mapping

`internal/controld/errors.go` centralizes all `controld-go` error handling
into a single function, `MapError`, called by every command right after an
API call fails. It's the only place in the codebase that classifies
`controld-go` errors into `ax.Error` values with exit codes.

| controld-go error     | HTTP status | ax exit code        | retryable |
| --------------------- | ----------- | ------------------- | --------- |
| `AuthorizationError`  | 401         | `ax.ExitAuth`       | no        |
| `AuthenticationError` | 403         | `ax.ExitAuth`       | no        |
| `NotFoundError`       | 404         | `ax.ExitValidation` | no        |
| `RequestError`        | 4xx         | `ax.ExitValidation` | no        |
| `RatelimitError`      | 429         | `ax.ExitNetwork`    | yes       |
| `ServiceError`        | 5xx         | `ax.ExitNetwork`    | yes       |

The `AuthorizationError`/`AuthenticationError` naming is a real gotcha:
despite the names, `controld-go` fires `AuthorizationError` on HTTP 401 (bad
or missing credentials) and `AuthenticationError` on HTTP 403 (valid
credentials, insufficient permission) — the two are swapped from what their
names suggest. `MapError`'s doc comment calls this out explicitly, and both
still map to `ax.ExitAuth` here regardless, so the swap only affects which
`actionable_fix` text is attached, not the exit code:

```go
var authzErr *controld.AuthorizationError // fires on HTTP 401
if errors.As(err, &authzErr) {
    return ax.NewError(ctx, "controld_authentication_failed", authzErr.Error(),
        ax.WithActionableFix("re-authenticate: check CONTROLD_API_TOKEN"),
        ax.WithRetryable(false),
        ax.WithErrorExitCode(ax.ExitAuth),
    )
}
```

Beyond the typed-error table, `MapError` also does a `strings.Contains`
match against two fixed substrings:

```go
const (
    vendorRateLimitRetriesExhausted = "exceeded available rate limit retries"
    vendorServiceUnavailableRetry   = "please try again later"
)
```

This exists because `controld-go`'s own request loop intercepts every HTTP
429 and 5xx response internally, retries a bounded number of times with
exponential backoff, and only after giving up returns a plain
`errors.New`/`fmt.Errorf` string — never a `*controld.RatelimitError` or
`*controld.ServiceError`. That means the `errors.As` checks for those two
types can never fire against a real HTTP response; they only pass in this
package's own unit tests, which hand-construct the typed errors directly.
The two `strings.Contains` checks are what actually classify a real
rate-limited or unavailable response correctly. The source comments call
this out as an explicitly fragile-but-contained workaround: if a future
`controld-go` version starts returning typed errors for these cases instead,
the string checks become redundant but harmless, and the substrings should
be reverified against that version's source before being relied on further.

## Safety primitives

Every mutating command (`create`, `update`, `delete`) wraps its actual API
call in `ax.Guard`:

```go
ran, err := ax.Guard(cmd.Context(), func(ctx context.Context) error {
    var createErr error
    result, createErr = client.CreateDevice(ctx, controld.CreateDeviceParams{...})
    return createErr
})
```

`ax.Guard` skips the callback entirely when `--dry-run` is active, returning
`(false, nil)` instead — the command still emits its response envelope, just
with `ran == false` and an empty/zero-value payload.

Every `delete` command adds a second gate in front of `ax.Guard`:
`ax.Confirm`, which requires `--yes` (or an interactive `[y/N]` prompt in
human mode) before the delete is allowed to proceed at all:

```go
outcome, err := ax.Confirm(cmd.Context(), subject)
if err != nil {
    return err
}
if outcome == ax.ConfirmationPromptRequired {
    approved, promptErr := promptForConfirmation(cmd, subject)
    ...
}
```

The ordering is deliberate: `ax.Confirm` runs first and can return an error
(`confirmation_required`, exit code `ax.ExitValidation`) before `ax.Guard`
ever runs. `--dry-run` alone does not bypass the confirmation requirement in
machine mode — a `delete` invoked with `--dry-run` but without `--yes` still
fails with `confirmation_required`, because confirmation is a separate gate
from the dry-run-skips-the-effect behavior `ax.Guard` provides. An agent (or
script) driving `controldctl` in machine mode must pass `--yes` explicitly to
get past a delete command, dry-run or not.

## Authentication

Every command that needs to call the ControlD API resolves an API token
through `internal/controld.NewClient`, in a fixed precedence order:

1. `--api-token` flag
2. `CONTROLD_API_TOKEN` environment variable
3. the `api_token` field of a Hujson file passed via `--config`

```go
func resolveAPIToken(ctx context.Context, apiTokenFlag, configPath string, getenv func(string) string) (string, error) {
    if apiTokenFlag != "" {
        return apiTokenFlag, nil
    }
    if token := getenv("CONTROLD_API_TOKEN"); token != "" {
        return token, nil
    }
    if configPath != "" {
        ...
    }
    return "", ax.NewError(ctx, "controld_no_api_token", ...)
}
```

If none of the three sources yields a token, the command fails immediately
with a `controld_no_api_token` error (`ax.ExitAuth`) rather than attempting a
request that would fail with a ControlD-side 401.

## The `--mcp` shim and client injection

`cmd/controldctl/main.go` rewrites a leading `--mcp` argument into the
`mcp-server` subcommand name before Cobra ever parses arguments:

```go
func rewriteMCPFlag(args []string) []string {
    if len(args) == 0 || args[0] != "--mcp" {
        return args
    }
    rewritten := make([]string, 0, len(args))
    rewritten = append(rewritten, "mcp-server")
    rewritten = append(rewritten, args[1:]...)
    return rewritten
}
```

This is a pure argument rewrite, not a second code path: `controldctl --mcp`
and `controldctl mcp-server` end up running the exact same `mcp.NewCommand`
implementation, so `--help` output and `__schema` never reference a flag
that the real `mcp-server` subcommand doesn't independently define.

The ControlD API client itself is constructed through a factory function
threaded through every command via closures, rather than a package-level
singleton:

```go
factory := func(cmd *cobra.Command) (*controld.API, error) {
    apiToken, _ := cmd.Flags().GetString("api-token")
    configPath, _ := cmd.Flags().GetString("config")
    return controld.NewClient(cmd.Context(), apiToken, configPath, "", os.Getenv)
}
```

Every `new*Command` constructor in `cmd/controldctl/` takes this
`clientFactory` and calls it inside `RunE`, which keeps client construction —
and its token resolution — deferred until a command actually runs, and gives
tests a seam to substitute a factory pointed at an `httptest.Server`.

## The MCP server

`root.AddCommand(mcp.NewCommand(root, ...))` in `main.go` mounts a reserved
`mcp-server` subcommand generated entirely from the same `root` Cobra command
tree the CLI itself uses. Every non-reserved command — everything except
`__schema`, `mcp-server`, and Cobra's own `completion` — is exposed as an MCP
tool; an MCP client's `tools/call` request runs that command in machine mode
and returns its JSON payload. There is no separate MCP-specific command
tree or handwritten tool schema to keep in sync — `ax-go` derives the tool
list, argument schema, and non-determinism metadata directly from the Cobra
commands and their flags. Adding a new `controldctl` subcommand under
`cmd/controldctl/` automatically adds a new MCP tool; nothing in
`internal/controld` or `main.go` needs to change for that to happen.
