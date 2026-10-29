## Context

See `proposal.md` — Why. The design constraints that actually shape the
approach:

- **The pinned client covers Phase A and nothing past it.** Read from the
  v0.0.10 source, not its README (the pin has since moved to v0.0.13; task 3.4
  re-verified it): `ListUser`, `ListIP`, `ListKnownIPs`,
  `LearnNewIPs`, `DeleteLearnedIPs`, `ListNetwork` exist and are typed. As of
  v0.0.13 upstream also types billing (`ListPayments`, `ListSubscriptions`,
  `ListActiveProducts`); there are still no organization or provisioning
  methods.
- **A soft fork already exists** at `github.com/rshade/controld-go`, with
  `module github.com/baptistecdr/controld-go` unchanged. Its `main` is synced
  to upstream v0.0.13 (0 ahead, 0 behind, `billing.go` present). It has no
  tags yet, and one branch,
  `fix/preserve-typed-errors-on-retry-exhaustion`, carries the typed-error fix.
- **`ax.Metadata` is closed, but the envelope is composable.**
  `contract.Metadata` carries exactly `trace_id`, `span_id`,
  `idempotency_key`, `dry_run` and has no extension point. That is not a
  constraint on the envelope, because `ax.Envelope[T]` is a type *alias* and
  `ax.WriteJSON` accepts `any` — a local metadata type can embed
  `ax.Metadata` and add fields, which JSON-promotes flat. `withTraceMetadata`
  is unexported, so a local envelope must re-wrap `ax.NewEnvelope`'s output
  rather than rebuild metadata.
- **Client-wide headers are already available.** `controld.Headers(http.Header)`
  is a construction-time option applied to every request.
- **Two response contracts are unverified; billing is confirmed.**
  `/organizations/*` and `/provision` are absent from the official reference
  entirely, and the probed account returned 404 "no organizations associated"
  for them. `/billing/*` publishes `"properties": {}` officially but is typed
  upstream as of v0.0.13, and the live probe confirmed upstream's structs cover
  every observed field. `/billing/products` returns the account's active
  products, not a subscribable catalog.
- **v1 conventions are load-bearing:** deterministic envelopes, `ax.Guard` for
  dry-run, `ax.Confirm` before destructive calls, casing wrappers for uppercase
  vendor JSON tags, `ax:"nondeterministic"` for server-generated values, and 39
  golden fixtures asserted byte-for-byte against an `httptest.Server`.

## Goals / Non-Goals

**Goals:**

- Land Phases A, B, and C behind one coherent command tree, with Phase C's
  unverified contracts isolated behind an explicit probe step rather than
  guessed at.
- **Invariant: nothing lands in the fork that cannot go upstream.** Every
  commit on `github.com/rshade/controld-go` must be a commit
  `baptistecdr/controld-go` could accept as-is. The fork is a delivery vehicle
  for upstream-bound work that has not merged yet — never a place to park
  controldctl-specific behavior. Anything that fails this test belongs in
  `internal/controld` or nowhere.
- Make the effective tenant unmistakable in machine output, since every command
  is also an MCP tool.

**Non-Goals:**

- No analytics-data commands. `/stats` and `/queries` appear only in community
  docs; the official reference has no such endpoints and ControlD's own forum
  carries an open request for the API. Building on them risks shipping against
  a private endpoint.
- No `analytics levels` / `analytics regions` catalog commands. They are typed
  in the pinned client but standalone-useless; revisit when a command needs to resolve a
  storage-endpoint PK interactively.
- No per-request impersonation. `--org-id` is a persistent flag with process
  lifetime; a single command never mixes tenants.
- No local caching of probe results at runtime. Probe findings are
  documentation that informs struct definitions, not data the binary reads.

## Decisions

### D1. Consume the fork via a `replace` to a published fork commit, not a module rename

`go.mod` keeps `require github.com/baptistecdr/controld-go` and adds
`replace github.com/baptistecdr/controld-go => github.com/rshade/controld-go <version>`,
where `<version>` is a published commit of the fork pinned as a pseudo-version
(currently `v0.0.0-20261028230926-59e7e774ad7e`, the head of
`fix/preserve-typed-errors-on-retry-exhaustion`). A tag such as
`v0.0.11-rshade.0` is a later cleanup, not a precondition. The fork keeps
upstream's module path.

*Why:* the fork's stated purpose in `docs/upstream-controld-go-gaps.md` is
preparing upstream PRs. Keeping the module path means every feature branch
diffs cleanly against `baptistecdr/controld-go` with no rename noise, and
returning to upstream after a merge is deleting one line rather than editing
imports across `internal/controld`.

*Alternative considered — rename the fork's module to
`github.com/rshade/controld-go` and `require` it directly.* This preserves
`go install <pkg>@latest`, which refuses any module whose `go.mod` carries a
`replace`. Rejected because it inverts the cost: every upstream PR then has to
strip a `go.mod` rename, and `internal/controld/client.go` plus `README.md`
plus `CONTEXT.md` all have to name the fork and later be reverted. The
documented install paths — `go build ./cmd/controldctl` and the GoReleaser
release binaries — are unaffected by a `replace`.

*Hard constraint:* the `replace` target must be a **published fork commit or
tag**, never a local filesystem path. The v1 build deliberately removed a local-path `replace`
and verified a fresh clone still builds; that property must survive.

### D2. The `org_id` echo lives in envelope metadata, via a local envelope type

A local metadata type embeds the closed `ax.Metadata` and adds one field; a
local envelope type carries it; one constructor re-wraps `ax.NewEnvelope` so
ax-go keeps owning trace, span, idempotency-key and dry-run:

```go
type meta struct {
    ax.Metadata        // embedded: JSON-promotes flat
    OrgID       string `json:"org_id"`
}

type envelope[T any] struct {
    Data T    `json:"data"`
    Meta meta `json:"meta"`
}

func newEnvelope[T any](ctx context.Context, data T) envelope[T] {
    e := ax.NewEnvelope(ctx, data)
    return envelope[T]{Data: e.Data, Meta: meta{Metadata: e.Meta, OrgID: orgFrom(ctx)}}
}
```

No `omitempty` — the spec requires the key present unconditionally, with `""`
meaning "the token's own account". Verified against ax-go v0.5.0: the embedded
metadata flattens, producing
`{"data":{…},"meta":{"trace_id":…,"span_id":…,"org_id":"org-42"}}`.

*Why metadata rather than the payload:* the effective organization is request
context — the same category as `trace_id` — not resource state. Two concrete
consequences follow. It cannot collide with a resource that has its own
organization identity, which `orgs show` and `orgs sub-orgs list` both do; an
echo embedded in those payloads would sit beside the org's own id in the same
object. And it stays out of `__schema` payload output and non-determinism
tagging, where it does not belong — the value is caller input, not
server-generated.

*Alternative considered — embed an `orgEcho` struct in all 24 payload types.*
Rejected: 24 struct edits plus their constructors instead of one helper and a
mechanical swap of 27 call sites, and it walks straight into the `orgs`
collision above.

*Alternative considered — extend `ax-go` with `Metadata.Extra` or
`ax.WithEnvelopeMetadata(ctx, "org_id", id)`, as the scoping doc requested.*
Unnecessary. Composition already achieves it without a cross-repo release, so
this is dropped rather than deferred.

*Alternative considered — echo only when impersonating.* Rejected: it leaves
v1 fixtures untouched (the entire appeal) but makes absence ambiguous — a
consumer cannot distinguish "not impersonating" from "this build predates the
echo".

*Consequence, accepted:* all 39 golden fixtures regenerate, each gaining
`meta.org_id`. Mitigated by the fact that
`go test ./cmd/controldctl -run TestGolden -update` regenerates them
mechanically and the diff is one added key per fixture, reviewable at a glance.

### D3. Phase B needs no fork change — use the client-wide `Headers()` option

`--org-id` is resolved in `internal/controld.NewClient` alongside the token and
applied as `controld.Headers(http.Header{"X-Force-Org-Id": []string{orgID}})`.

*Why:* the flag is persistent and process-wide, so a client-wide header
expresses it exactly. The gaps doc's proposed per-request `WithHeader` option
is a genuine upstream improvement but buys this change nothing, and adding
unused API to the fork is fork debt.

*Alternative considered — add `WithHeader` to the fork now.* Rejected as
YAGNI; it stays an upstream-PR item in `docs/upstream-controld-go-gaps.md`.

### D4. The impersonation mutation gate is a shared helper, not per-command policy

`ax-go` v0.5.0 invokes `ax.Confirm` manually inside each command and has no
policy hook. Rather than duplicating an `if orgID != ""` branch in every leaf,
a single helper wraps the decision:

```go
// requireConfirmation reports whether this mutation must be confirmed:
// always for destructive verbs, and for every verb under impersonation.
func requireConfirmation(cmd *cobra.Command, destructive bool) bool
```

Every mutating command calls it in place of the current bare `destructive`
check, so the rule lives in one place and a new command cannot forget it.

*Alternative considered — an `ax.WithConfirmationPolicy(...)` hook upstream.*
Unlike D2's metadata request, this one cannot be composed around: `ax.Confirm`
is called inside each command, so there is no seam to wrap. It stays a genuine
`ax-go` follow-up, and the helper is the local stand-in until it exists.

### D5. Phase C is gated on a live probe, and the probe's output is a committed document

Phase C structs (billing, via upstream; orgs and provisioning, moved to #32 and #33) are not written from guesses. A gated probe step runs against a
live business-tier account, records each endpoint's real response shape, and
writes findings into `docs/upstream-controld-go-gaps.md` (extending the
existing "Investigation Needed" bullets it already carries). Only then are the
fork's typed structs written.

The probe follows the pattern the roadmap already proposes for end-to-end
smoke tests: skipped unless a token env var is set, never run in CI, and it
records shapes rather than asserting values. Response bodies containing account
identifiers, emails, or payment details are redacted before anything is
committed.

For billing the probe is a smoke check, not shape discovery: upstream already
types it. It confirmed reachability (HTTP 200 on all three endpoints), that live
responses decode into upstream's structs, and that `/billing/products` returns
active products. `/organizations/*` and `/provision` returned 404 "no
organizations associated" from an account without an organization, so they
could not be probed; that work moved to issues #32 and #33.

*Why:* `/organizations/*` and `/provision` are not in the official reference at
all. Writing structs first would mean shipping plausible-looking fields that
silently decode to zero values.

*Alternative considered — model Phase C payloads as `map[string]any` and pass
the API's JSON through untouched.* Rejected: it defeats the casing-normalization
and non-determinism-tagging requirements in the specs, and produces envelopes
whose shape varies by account.

### D6. Phase C uses typed fork methods, never `API.Raw`

A group upstream has not typed gets a file in the fork, written to upstream's
existing shape: a `XxxResponse` struct embedding `Response`, `buildURI`,
`makeRequestContext`, and a table test with `testdata` fixtures, so each file is
PR-ready as-is. A group upstream has already typed (billing, as of v0.0.13) is
consumed, not re-implemented: duplicating it would put fork-only code in the
fork and collide on the next rebase. Organizations and provisioning are the
untyped groups; their fork files (`organizations.go`, `provision.go`) are
tracked in issues #32 and #33, not in this change.

*Why:* `docs/upstream-controld-go-gaps.md` §1 states a "No Raw Drifting"
policy explicitly. Owning response structs inside `controldctl` would put
vendor-shaped types in the wrong repo and drift from any future upstream
support.

*Binding test for every fork commit — the invariant in Goals:* would
`baptistecdr/controld-go` accept this commit as-is? Concretely that means each
one is a general ControlD-API capability (not a controldctl need), carries
upstream's own file layout, naming, table tests and `testdata` fixtures, adds no
reference to `controldctl` or `ax-go`, and is opened as an upstream PR at the
same time it is pushed to the fork — not "later, if there is time".

*Tagging discipline that keeps that true:* the fork's `main` tracks upstream.
Each group is a branch off `main` that becomes one upstream PR. The tag
`controldctl` consumes is cut from a branch that is exactly the merge of those
PR branches and nothing else — so the tag is fork-only, but every commit it
contains is upstream-bound. A fork-only tag is packaging; fork-only *code* is
what the invariant forbids.

*Bundled while the PR channel is open — gaps §3.1:* the retry loop collapses an
exhausted 429 to `errors.New("exceeded available rate limit retries")` and an
exhausted 5xx to a `please try again later` string, discarding the
`*RatelimitError` / `*ServiceError` it already built for the non-exhausted path.
`controldctl` works around that with substring matching in
`internal/controld/errors.go`. The fix wraps rather than replaces — same message
text, typed error reachable through `%w` — so it is non-breaking even for
callers matching on strings, which is what makes it an easy upstream accept. It
is not required by this change; it is bundled because it deletes a fragile
workaround here.

*Sequencing consequence:* §3.1 is verified from source, so it is not gated on
the probe. That decouples the `go.mod` `replace` from Phase C: the fork is
worth consuming for the typed-error fix alone. The `replace` and the workaround
deletion therefore live in the fork task group, not in Phase C.

*Ownership:* the fork changes are authored as part of this change, but pushing
branches, opening the upstream PRs, and cutting the consumed tag are the
maintainer's. No task may run `git push`, `gh pr create`, or `git tag`.

*Moved with organizations (#32):* if organization mutations require
`application/x-www-form-urlencoded`, the fix is opt-in form-encoding in the
fork's request layer, written as a general capability and never worked around in
`controldctl`.

### D7. Provisioning-code redaction — moved to issue #33

Redacting the provisioning-code value at payload construction, behind a fixed
constant marker unless a reveal flag is passed, moved out of this change with
the `provision` commands. The decision and its rationale are carried in #33.

### D8. Command tree follows API routes, at top level

`users`, `ip`, `access`, `network`, `billing` (and later `orgs`, `provision`)
are all top-level, mirroring `/users`, `/ip`, `/access`, `/network`,
`/billing/*` (and `/organizations/*`, `/provision`). Notably `access` is **not** nested under
`devices` even though every call takes a device identifier.

*Why:* it is the convention the approved scoping doc resolved, and it keeps the
mapping from an API reference page to a command mechanical — which matters
disproportionately for the MCP surface, where an agent reasoning from ControlD's
docs should be able to guess the tool name.

## Risks / Trade-offs

- **`replace` breaks `go install <pkg>@latest`** → The documented install paths
  (`go build`, release binaries) are unaffected. Add an explicit README note
  saying to build from a clone or download a release while the `replace` is in
  place, and delete both the note and the `replace` when upstream merges.
- **All 39 golden fixtures change in one commit** → Regenerate mechanically with
  `-update`, and review the diff for exactly one added `"org_id":""` key inside
  each fixture's `meta` object and nothing else. Any other delta in that diff is
  a bug.
- **Phase C cannot be verified without a business-tier account** → If no such
  account is available, Phase C stops at the probe task with its findings
  recorded, and Phases A and B still ship complete. This is the most likely
  place for this change to partially land; the task breakdown is ordered so
  that outcome is clean rather than half-finished.
- **Community-documented paths may not exist or may change** → The probe
  detects this immediately (404 rather than a shape), and the finding is
  recorded instead of a command being shipped against a phantom endpoint.
- **Fork maintenance** → The fork must be rebased on upstream releases and
  re-tagged. Kept small by D3 (no `WithHeader`) and bounded by the Goals
  invariant: because nothing fork-only is ever added, a rebase can only ever
  conflict with upstream work on the same files, and each PR that merges
  *shrinks* the fork rather than growing the drift.
- **The stronger impersonation gate can surprise existing automation** → It only
  triggers when `--org-id` is in effect, which no existing script passes,
  because the flag does not exist yet. Unimpersonated behavior is byte-identical.
- **Financial and secret data reachable by an autonomous agent** → Billing help
  text declares sensitivity. That is not a substitute for the operator scoping the token they hand an agent, and the docs
  should say so.

## Migration Plan

Sequential, with each phase independently shippable and independently
revertible:

1. **Phase A** — six commands on the pinned upstream client. No `go.mod` change, no
   fixture churn beyond the new commands' own fixtures. Revert = delete files.
2. **Phase B** — `--org-id`, the echo, the gate. This is the fixture-regenerating
   commit; keep it alone so the 39-fixture diff is not mixed with feature work.
   Revert = revert one commit.
3. **Probe** — run against a live business-tier account, record findings, commit
   the documentation. No code.
4. **Fork work** — typed methods per group, tagged release, upstream PRs opened
   in parallel.
5. **Phase C** — the billing commands over upstream's typed methods. The
   `go.mod` `replace` lands with the typed-error fix in the fork task group, not
   here. Organizations and provisioning moved to #32 and #33.

Rollback for the `replace`: dropping back to the pinned upstream version means
reverting the typed-error workaround deletion (task 4.9) with it. The billing
commands do not depend on the fork and stay.

## Open Questions

These can be answered later without changing the specs, the approach, or the
task breakdown:

- **Which token scope the billing endpoints require.** ControlD issues Read
  and Write tokens, and whether billing reads need a particular scope or account
  tier is unknown; the probe harness did not record the scope it ran with. The
  answer becomes a line of per-command help text, not a design change.
- **Whether upstream accepts the typed-error PR, and on what timeline.** Does
  not gate anything here — the fork tag is the dependency, and upstream acceptance
  only determines when the `replace` can be deleted.
