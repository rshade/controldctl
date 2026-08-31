# controldctl Strategic Roadmap

## Vision

A CLI and MCP server for ControlD's DNS filtering API, with identical
behavior for human and LLM-agent callers. See [CONTEXT.md](CONTEXT.md) for
architectural boundaries. Core DNS control (devices, profiles, filters,
services, custom rules, rule folders) shipped as the initial build; this
roadmap tracks hardening and the deferred scope from that build.

## Immediate Focus

- [ ] #1 Add CI workflow (build, vet, gofmt, race-tested tests) [S]
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
  See `docs/superpowers/plans/2026-08-29-controldctl-cli-mcp.md` (local,
  untracked — see Boundary Safeguards) for the full build record. No
  tracking issue — predates issue-based tracking on this repo. Closed
  2026-08-31.

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
