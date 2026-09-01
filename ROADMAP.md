# controldctl Strategic Roadmap

## Vision

A CLI and MCP server for ControlD's DNS filtering API, with identical
behavior for human and LLM-agent callers. See [CONTEXT.md](CONTEXT.md) for
architectural boundaries. Core DNS control (devices, profiles, filters,
services, custom rules, rule folders) shipped as the initial build; this
roadmap tracks hardening and the deferred scope from that build.

## Immediate Focus

- [ ] #2 Makefile lint target can't fail on formatting issues [S]

## Near-Term Vision

- [ ] #3 Backfill golden-fixture (testdata/) tests matching ax-go convention [M]
- [ ] #4 Add real end-to-end smoke-test tier gated on CONTROLD_API_TOKEN [M]
- [ ] #5 Fix t.Fatal-in-goroutine pattern in httptest handlers across test files [S]

## Future Vision (Long-Term)

- [ ] #6 Scope and design the Account/Org resource surface (Users, Billing, Network Stats, Access/IP logs, org impersonation) [L]
- [ ] #7 Add deferred optional flags per resource (DDNS, legacy-IPv4, lock_status, via/group/order) [M]
- [ ] #8 Normalize dry-run "nothing happened" payload shape across resources [S]
- [ ] #9 Improve validation-error actionable_fix coverage (1/20 currently) [S]
- [ ] #10 README polish: mention --idempotency-key, hint at rules create's --folder-id=0 default [S]

## Completed Milestones

### 2026-Q3

- [x] `controldctl`: initial build — devices, profiles (options/filters/
  services/rules/folders), `mcp-server`/`--mcp`, `__schema`, full CRUD with
  `--dry-run`/`--yes` safety gates. Built via a 10-task plan plus a final
  whole-branch review and one fix wave; every task individually reviewed.
  The full build record is kept locally and gitignored (see Boundary
  Safeguards), not part of this repo's tracked history. No tracking
  issue — predates issue-based tracking on this repo. Closed 2026-08-31.
- [x] Post-v1 infrastructure: renamed the module to `controldctl`, moved
  to the standard `cmd/controldctl/` layout, pinned `ax-go` to the
  published v0.5.0 release (dropped a local-path `replace`, verified
  with a genuine fresh clone that the repo now builds standalone), added
  `docs/architecture.md`/`docs/CONTRIBUTING.md`/`docs/commands.md`, and
  set up Renovate, mise, release-please, and GoReleaser — the last of
  which surfaced and fixed a real bug: `-ldflags` version injection was
  never actually wired up in `main.go`, so every prior build reported a
  placeholder version regardless of build flags. No tracking issue —
  done directly, same session as the initial build. Closed 2026-08-31.
- [x] CI workflow: added `.github/workflows/ci.yml` running go build
  (`CGO_ENABLED=0`), go vet, a gofmt check, golangci-lint, govulncheck, and
  actionlint against the workflow itself, plus race-tested tests, on every
  push to `main` and pull request, with concurrency cancellation and a Go
  module/build cache. Closes #1. Closed 2026-08-31.

## Boundary Safeguards

From [CONTEXT.md](CONTEXT.md) — do not build past these without a fresh
scoping decision:

- No local DNS resolution/proxying (that's ControlD's own `ctrld`, a
  different tool).
- No Users/Billing/Network Stats/Access-IP-logs/org-impersonation/
  mass-provisioning without a scoping pass (tracked as #6).
- No local persistent state — the ControlD API is the sole source of truth.
- No direct vendor-library or raw-HTTP-client calls from command files —
  everything routes through `internal/controld`.
