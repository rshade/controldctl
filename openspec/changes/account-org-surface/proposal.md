## Why

v1 shipped "Core DNS control" and explicitly deferred ControlD's account/org
surface — account info, access/IP logs, network status, billing, organizations,
org impersonation, and mass provisioning — as a candidate for its own scoping
pass. That pass is done and approved
(`docs/superpowers/specs/2026-09-01-account-org-scope.md`, issue #6): it
inventories every resource group against the pinned
`baptistecdr/controld-go` source (v0.0.10 when scoped; the pin has since moved
to v0.0.13), maps each to an official or
community-documented endpoint, and phases the work by library support. This
change implements the full scoped surface, so that everything an operator can
reach in the ControlD dashboard is reachable from the CLI and — because every
command becomes an MCP tool automatically — from an LLM agent, with the same
envelopes and the same safety gates as v1.

## What Changes

Delivered in three phases (all in this change), ordered by library support:

**Phase A — typed methods already in the pinned client (verified at v0.0.10), no dependency work:**

- `controldctl users show` (`GET /users`) and `controldctl ip show`
  (`GET /ip`) — account record and the caller's egress IP as ControlD sees it.
- `controldctl access list|learn|delete` (`GET|POST|DELETE /access`) — the
  known/learned IP allowlist that authorizes a device's resolver.
- `controldctl network list` (`GET /network`) — ControlD's own PoP/resolver
  health. Named for the API route; this is *not* per-account query statistics.

**Phase B — cross-tenant impersonation:**

- A persistent `--org-id` flag sending `X-Force-Org-Id`, resolved
  flag > `CONTROLD_ORG_ID` > config `org_id`, mirroring token precedence.
- **BREAKING to the v1 confirmation rule, deliberately:** when `--org-id` is
  active, *every* mutation requires `--yes` in machine mode, not just deletes.
  Cross-tenant blast radius justifies the stronger gate. Unimpersonated
  behavior is unchanged.
- **BREAKING to the envelope shape:** the effective `org_id` is echoed
  unconditionally in *every* success envelope's `meta` object — including all
  v1 commands, and including an empty value meaning "the token's own account" —
  so machine consumers can assert which tenant they hit without inspecting
  their own config. This regenerates all 39 existing golden fixtures.

**Phase C — billing, typed upstream as of v0.0.13 (re-verified, task 3.4):**

- `controldctl billing payments|subscriptions|products` (`GET /billing/*`),
  over upstream's own `ListPayments`, `ListSubscriptions` and
  `ListActiveProducts`, available once the fork is synced to v0.0.13.

**Moved out of this change:** `controldctl orgs` (`/organizations/*`, issue
#32) and `controldctl provision` (`/provision`, issue #33). The live probe got
HTTP 404 "You have no organizations associated with your account" from all four
endpoints because the probed account has no organization, so their payload
shapes are unknown and cannot be typed. Both are blocked on an
organization-enabled token and are tracked, with their requirements, in those
issues.

**Probe result for billing:** `/billing/*` publishes empty response schemas in
ControlD's official reference but is already typed upstream. The live probe
returned HTTP 200 for all three endpoints, every observed field is covered by
upstream's structs, and `/billing/products` returns the account's active
products rather than a subscribable catalog. Phase C requirements below
constrain command shape, safety, and envelope discipline — not field-level
payload contents.

## Capabilities

### New Capabilities

- `account-info`: Read-only access to the ControlD account record
  (`users show`) and the caller's egress-IP view (`ip show`).
- `access-ips`: List, learn, and delete the known IPs that authorize a
  device's resolver, including the delete-safety gate.
- `network-status`: Read-only view of ControlD's PoP/resolver health.
- `org-impersonation`: The `--org-id` flag, its resolution precedence, its
  unconditional envelope echo, and the all-mutations-require-`--yes` gate.
- `billing`: Read-only payment history, subscriptions, and active products.

### Modified Capabilities

(none — `ci-workflow` is the only existing spec and its requirements are
unchanged.)

## Impact

- **Boundary documents:** `CONTEXT.md`'s "Hard No's" and `ROADMAP.md`'s
  "Boundary Safeguards" both currently forbid this entire surface pending a
  scoping decision. That decision is issue #6 and it is made — both files must
  be updated to reflect the new boundary, not silently contradicted.
- **New code:** five command files under `cmd/controldctl/`, matching
  adapter re-exports and billing wrappers in `internal/controld/`, and
  `--org-id` plumbing through `internal/controld.NewClient` and `root.go`.
- **Dependency:** `go.mod` gains a `replace` directive pinning
  `github.com/baptistecdr/controld-go` to a published
  `github.com/rshade/controld-go` commit (a pseudo-version; a tag is optional
  cleanup). This is the first `replace` since the v1 build deliberately removed
  one, and it must point at a published fork version
  — never a local path — so the repo still builds from a fresh clone.
- **Fork work:** a fix preserving typed `*RatelimitError` / `*ServiceError`
  through the retry loop (`docs/upstream-controld-go-gaps.md` §3.1), written to
  upstream's own conventions so it is PR-ready. Billing is already upstream, and
  organizations and provisioning fork work moved to #32 and #33. Phase B needs no fork change: `--org-id` is
  a persistent flag, so the client-wide `Headers()` option already in the pinned client
  expresses it exactly. Branch pushes, upstream PRs, and the consumed tag are
  the maintainer's to make, not this change's.
- **Workaround removed:** with §3.1 fixed upstream, the substring-matching
  fallbacks in `internal/controld/errors.go` and their two vendor-message
  constants are deleted; `MapError`'s existing `errors.As` branches then fire
  on real exhausted retries instead of only in hand-constructed tests.
- **MCP:** every new command becomes an MCP tool with no extra work, which is
  precisely why the impersonation echo and the billing-sensitivity note are
  requirements rather than niceties.
- **Docs:** `docs/commands.md` gains the new command tree;
  `docs/upstream-controld-go-gaps.md` is updated as gaps are closed in the fork.
- **Unchanged:** `internal/controld/errors.go`'s `MapError` (fork calls produce
  the same error taxonomy), envelope/`--format` conventions, and the
  no-local-state and no-vendor-imports-from-command-files boundaries.
