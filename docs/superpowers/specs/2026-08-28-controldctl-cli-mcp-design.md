# controldctl: a CLI and MCP server for the ControlD API

**Status:** Approved for implementation planning
**Date:** 2026-08-28

## Mission

Build `controldctl`, a Go CLI that manages ControlD (controld.com) DNS
filtering resources — devices, profiles, filters, services, custom rules, and
rule folders — and exposes the exact same functionality as a live MCP server
with no per-command work, by building on
[`ax-go`](https://github.com/rshade/ax-go) (the "Agentic Experience"
foundation for the rshade portfolio) and adopting
[`baptistecdr/controld-go`](https://github.com/baptistecdr/controld-go) as the
underlying ControlD API client.

## Background

- `ax-go` standardizes how Go CLIs behave for both humans and LLM agents:
  deterministic JSON envelopes on stdout, structured `ax.Error` on stderr,
  `--dry-run`/`--yes`/idempotency-key safety primitives, and a `__schema`
  command for machine discovery. Its `mcp` package
  (`mcp.NewCommand`/`mcp.Serve`) walks an existing Cobra command tree and
  derives a live MCP server from it — the same command tree powers both the
  CLI and the MCP server, with zero additional per-tool wiring.
- `baptistecdr/controld-go` (MIT license, cloudflare-go-style) already
  implements the full v1 API surface we need: Devices (list/create/update/
  delete/types), Profiles (list/create/update/delete), Profile Options,
  Filters, Services, Custom Rules (full CRUD), and Rule Folders (full CRUD).
  It exposes typed errors (`AuthenticationError`, `AuthorizationError`,
  `NotFoundError`, `RequestError`, `RatelimitError`, `ServiceError`) and a
  `BaseURL` client option that lets tests point the client at an
  `httptest.Server` instead of the real ControlD API.
- ControlD's own official DNS proxy client is named `ctrld`
  (github.com/Control-D-Inc/ctrld) — a different tool for a different purpose
  (a local DNS forwarder), not a REST API client. We avoid `controld`/`ctrld`
  as our binary name to prevent confusion with it.

## Decisions

1. **API client:** depend on `baptistecdr/controld-go` rather than hand-rolling
   a REST client or generating one from an OpenAPI spec. It already covers the
   agreed v1 scope, handles auth/rate-limiting/retries, and its typed errors
   map cleanly onto `ax-go`'s exit-code taxonomy. Risk: it's a small,
   unofficial library (low star count, not maintained by ControlD). Mitigation:
   MIT license permits vendoring/forking if it stalls, and our own code only
   depends on it through the narrow `internal/controld` adapter boundary
   below, so replacing it later does not require reworking the command tree.
2. **v1 API scope — "Core DNS control":** Devices/Endpoints, Profiles, and
   their sub-resources (Options, Filters, Services, Custom Rules, Rule
   Folders). Out of scope for v1: Users/account info, Billing, Network Stats,
   Access/IP logs, org impersonation (`X-Force-Org-Id`), and mass
   provisioning. These become a later phase's own scoping pass if pursued.
3. **v1 operations — read + write:** full CRUD wherever `controld-go`
   supports it, gated by `ax-go`'s safety primitives (see below) rather than
   deferring mutations to a v2.
4. **Binary name:** `controldctl`.
5. **MCP invocation:** `controldctl mcp-server` is the real, ax-go-documented
   subcommand (reserved name, built by `mcp.NewCommand`) — it owns discovery,
   dispatch, and the loopback-only HTTP safety gate. `controldctl --mcp` is a
   pure convenience alias: `main()` rewrites `--mcp` into the `mcp-server`
   subcommand argument before Cobra parses, so there is exactly one code path
   underneath and `--help`/`__schema` output stays accurate (no flag ever
   appears that `mcp-server`'s own Cobra definition doesn't already have).

## Architecture

Single Go module, single binary, `go 1.26` (matching `ax-go`). `package main`
is split across files by resource rather than one large file, since the
command count (~23 leaf commands) is larger than `ax-go`'s own
`examples/integration` reference:

```
controld-go-mcp/
├── go.mod                          module github.com/rshade/controld-go-mcp
├── main.go                         entrypoint; --mcp arg-rewrite shim
├── root.go                         newRootCommand: global flags, ax.Execute wiring
├── devices.go                      devices list/create/update/delete/types
├── profiles.go                     profiles list/create/update/delete
├── profiles_options.go             profiles options list/update
├── profiles_filters.go             profiles filters list/update
├── profiles_services.go            profiles services list/update
├── profiles_rules.go               profiles rules list/create/update/delete
├── profiles_folders.go             profiles folders list/create/update/delete
├── internal/
│   └── controld/
│       ├── client.go                client construction; auth/config precedence
│       ├── errors.go                controld-go error -> ax.Error mapping
│       └── payloads.go              controld-go structs -> ax.Envelope payloads;
│                                     ax:"nondeterministic" tagging
└── testdata/                        golden fixtures (mirrors ax-go's convention)
```

`internal/controld` has no Cobra dependency and is unit-testable in isolation;
the command files are thin Cobra wiring that call into it.

`controld-go` exposes native and external filter categories as two separate
calls (`ListProfileNativeFilters`, `ListProfileExternalFilters`); `profiles
filters list` covers both through a `--source=native|external` flag
(default `native`) rather than two separate leaf commands, since they share
the same shape and the same `update` counterpart.

### Command tree

Resource-then-verb (kubectl-style), with sub-resources nested under `profiles`
to match how ControlD's own API paths nest them:

```
controldctl
├── devices          list · create · update · delete · types
├── profiles         list · create · update · delete
│   ├── options      list · update
│   ├── filters      list · update
│   ├── services     list · update
│   ├── rules        list · create · update · delete   (custom rules)
│   └── folders      list · create · update · delete   (rule folders)
├── mcp-server        (from ax-go's mcp.NewCommand; stdio default, optional HTTP)
└── __schema          (from ax-go, automatic)
```

## Auth and configuration

Token precedence, matching the layered pattern `ax-go`'s own example
demonstrates:

1. `--api-token` flag (highest precedence)
2. `CONTROLD_API_TOKEN` environment variable
3. `api_token` field in a Hujson config file (`--config`, read via
   `ax.ParseConfigFile`)

No org-impersonation header (`X-Force-Org-Id`) in v1 — that belongs to the
deferred account/org scope.

## Error handling

`internal/controld/errors.go` maps every `controld-go` error type to an
`ax.NewError` call once, so every command gets consistent exit codes and
retry semantics for free:

| controld-go error            | HTTP  | exit code         | retryable | notes                        |
| ----------------------------- | ----- | ------------------ | --------- | ----------------------------- |
| `AuthenticationError`         | 401   | `ax.ExitAuth` (4)  | false     | re-authenticate               |
| `AuthorizationError`          | 403   | `ax.ExitAuth` (4)  | false     | token lacks required scope    |
| `NotFoundError`                | 404   | `ax.ExitValidation` (2) | false | check the resource ID          |
| `RequestError` (other 4xx)    | 4xx   | `ax.ExitValidation` (2) | false | bad params                     |
| `RatelimitError`               | 429   | `ax.ExitNetwork` (3) | true    | `retry_after_seconds` set      |
| `ServiceError` / network error | 5xx / timeout | `ax.ExitNetwork` (3) | true | upstream/connectivity failure |

Any error type not covered by this table (a new `controld-go` release adding
one) falls back to `ax.ExitInternal` (1) via `ax.Execute`'s default handling
of an unwrapped error, so the mapping table degrading gracefully does not
require an exhaustive type switch to stay safe.

## Agent-safety primitives

- Every `delete` command requires `--yes` (`ax.Confirm`) before it executes a
  real deletion.
- Every mutating command (`create`/`update`/`delete`) honors `--dry-run` via
  `ax.Guard` (skip-only: the mutating API call is skipped and the same
  envelope shape is returned with `dry_run: true`). Unlike `ax-go`'s
  Hujson-patch example, there is no safe local rehearsal for a remote DNS
  policy change, so `ax.Perform`'s validate-then-commit pattern does not
  apply here — `ax.Guard`'s simpler skip-only semantics are the right fit.
- `--idempotency-key` is available on every command per `ax-go` defaults, most
  meaningful on `create` operations to protect against duplicate-create from
  agent retries.

## Testing

- `internal/controld`'s error- and payload-mapping functions are pure and
  unit-tested directly, no network involved.
- Command-level tests construct the `controld-go` client with its `BaseURL`
  option pointed at an `httptest.Server` serving fixture JSON, so the full
  Cobra command executes against a fake ControlD API with no real network
  call. Golden fixtures for stdout envelopes live under `testdata/`, mirroring
  `ax-go`'s own convention.
- Non-deterministic response fields (`Device.Ts`, server-generated IDs
  returned from `create` calls) are tagged `ax:"nondeterministic"` and
  registered via `ax.WithNonDeterministicFields`, so golden-fixture
  comparisons stay stable across runs.
- An optional manual smoke-test tier runs the built binary against a real
  ControlD account when `CONTROLD_API_TOKEN` is set locally. It never runs in
  CI.

## Out of scope for v1

Users/account info, Billing, Network Stats, Access/IP logs, org impersonation
(`X-Force-Org-Id`), and mass provisioning. Each is a candidate for its own
future scoping pass, not an assumed v2 — ControlD's org/account surface is
large enough to warrant a separate design conversation once Core DNS control
has shipped.
