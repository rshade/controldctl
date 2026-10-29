# Upstream controld-go Gap Backlog & Fork Plan

**Upstream Repository:** [`github.com/baptistecdr/controld-go`](https://github.com/baptistecdr/controld-go)  
**Pinned Version:** `v0.0.10`, consumed through the fork &mdash; see [Fork Consumption](#fork-consumption)  
**Purpose:** Track missing features, API endpoints, and bug fixes in the upstream client library to prepare contributions (pull requests) or maintain a temporary fork (`github.com/rshade/controld-go`).

---

## 1. Upstream Strategy

`controldctl` isolates all vendor interactions behind `internal/controld`. To ensure maintainability and broad ecosystem compatibility:

1. **Upstream First:** Where ControlD provides documented REST endpoints not yet supported in `controld-go`, we prepare typed Go structs and client methods to contribute upstream via pull requests.
2. **Local Fork Seam:** If upstream acceptance is delayed, we can point `go.mod` to a fork repository without modifying any command files under `cmd/controldctl/`.
3. **No Raw Drifting:** Avoid implementing ad-hoc `API.Raw` wrappers in `controldctl` for missing resources. All API surface extensions should exist as first-class, typed methods in the client library.

### Fork Consumption

The seam in point 2 is now in use. `go.mod` carries:

```text
replace github.com/baptistecdr/controld-go => github.com/rshade/controld-go v0.0.0-20261028230926-59e7e774ad7e
```

The fork keeps upstream's module path, so this is a single line and no source
file refers to the fork. The pseudo-version pins an immutable commit
(`59e7e77` on `fix/preserve-typed-errors-on-retry-exhaustion`, which is
upstream `v0.0.13` plus the §3.1 fix), so it does not move if the branch is
rewritten. No fork tag is cut; a tag is optional cleanup.

**Cost:** `go install <pkg>@latest` refuses any module whose `go.mod` has a
`replace`. Building from a clone and the release binaries are unaffected.

**Exit condition:** when upstream ships a release containing §3.1, delete the
`replace` line and the matching README note together.

---

## 2. Missing Resource Groups & Methods

### 2.1 Organizations API (`/organizations/*`)

- **Endpoints:**
  - `GET /organizations/organization` &mdash; Account organization details
  - `GET /organizations/members` &mdash; Org member list
  - `GET /organizations/sub_organizations` &middot; `GET /organizations/suborg` &mdash; Sub-organizations
  - `POST /organizations/*` &middot; `DELETE /organizations/*` &mdash; Member / sub-org management
- **Required Upstream Methods:**
  - `GetOrganization(ctx context.Context) (Organization, error)`
  - `ListMembers(ctx context.Context) ([]Member, error)`
  - `ListSubOrganizations(ctx context.Context) ([]SubOrganization, error)`
  - `CreateSubOrganization(ctx context.Context, params CreateSubOrgParams) (SubOrganization, error)`
- **Probe findings (task 3.2):** all three reads returned HTTP 404 with
  `You have no organizations associated with your account`. The message is
  specific rather than a generic not-found, which suggests the paths exist and
  the probed account simply has no organization; that is not yet proven. Field
  shapes are still unknown. The token used had write scope.
- **Decision (task 3.3): not reachable from the probed account, deferred**
  (HTTP 404). Tracked in issue #32, blocked on an organization-enabled token.
- **Investigation Needed:**
  - Verify whether mutation endpoints accept `application/json` or expect `application/x-www-form-urlencoded`. `controld-go`'s internal request loop currently defaults to JSON serialization.

### 2.2 Billing API (`/billing/*`)

- **Endpoints:**
  - `GET /billing/payments` &mdash; Payment history
  - `GET /billing/subscriptions` &mdash; Active subscription details
  - `GET /billing/products` &mdash; Product and tier listings
- **Required Upstream Methods:**
  - `ListPayments(ctx context.Context) ([]Payment, error)`
  - `ListSubscriptions(ctx context.Context) ([]Subscription, error)`
  - `ListProducts(ctx context.Context) ([]Product, error)`
- **Re-verified against upstream `v0.0.13`:** upstream now ships `billing.go`
  with typed `ListPayments`, `ListSubscriptions` and `ListActiveProducts` (the
  last backs `GET /billing/products`) and concrete `Payment`, `Subscription` and
  `Product` structs. The product method name differs from the one above, and the
  fork's pinned commit (`v0.0.0-20260908000437-10e038e0dafb`) predates it, so
  the fork does not expose these yet. Organizations and provisioning remain
  absent from `v0.0.13`.
- **Probe findings (task 3.2):** `GET /billing/payments`,
  `/billing/subscriptions` and `/billing/products` all returned HTTP 200 with
  `success=true`. Every field the probe observed is covered by upstream's
  `Payment`, `Subscription` and `Product` structs. `/billing/products` returns
  the account's active products (each carries a `subscription` object and an
  `expiry`), not a subscribable catalog, so `ListActiveProducts` is the right
  upstream method; the billing spec was reworded to "Active Product Listing".
  The token used had write scope, which sufficed; whether read scope suffices was not tested.
- **Decision (task 3.3): confirmed, proceed.** Shipped as `controldctl billing
  payments|subscriptions|products` over upstream's methods, with no fork code.
- **Investigation Needed:**
  - ControlD's OpenAPI definition publishes empty schemas (`"properties": {}`) for these endpoints. Response bodies must be probed against a live account with billing history to define typed Go structs.

### 2.3 Mass Provisioning Codes (`/provision`)

- **Endpoints:**
  - `GET /provision` &mdash; List provisioning codes
  - `POST /provision` &mdash; Create new provisioning code
  - `DELETE /provision/{code}` &mdash; Delete provisioning code
  - `PUT /provision/{code}/invalidate` &mdash; Invalidate code
- **Required Upstream Methods:**
  - `ListProvisioningCodes(ctx context.Context) ([]ProvisioningCode, error)`
  - `CreateProvisioningCode(ctx context.Context, params CreateProvisioningCodeParams) (ProvisioningCode, error)`
  - `DeleteProvisioningCode(ctx context.Context, code string) error`
  - `InvalidateProvisioningCode(ctx context.Context, code string) error`
- **Probe findings (task 3.2):** `GET /provision` returned HTTP 404 with
  `You have no organizations associated with your account`, the same response
  as §2.1. The probed account has no organization, so provisioning is also
  unreachable from it. Field shapes are unknown. The token used had write scope.
  recorded.
- **Decision (task 3.3): not reachable from the probed account, deferred**
  (HTTP 404). Tracked in issue #33, blocked on an organization-enabled token.

---

## 3. Upstream Library Bugs & Enhancements

### 3.1 Preserve Typed Errors for 429 and 5xx

- **Current Behavior in v0.0.10:**
  In `controld.go`'s `request()` method, rate-limit retries (HTTP 429) and server errors (HTTP 5xx) are retried with exponential backoff. When retries are exhausted, the client returns string errors:
  - `"exceeded available rate limit retries"`
  - `"please try again later"`
  The original `*controld.RatelimitError` or `*controld.ServiceError` is swallowed. `controldctl` currently works around this using `strings.Contains` matching in `internal/controld/errors.go`.
- **Proposed Upstream Fix:**
  Wrap or preserve the terminal `*RatelimitError` or `*ServiceError` so callers can inspect them using standard `errors.As`.
- **Status: fixed in the fork, consumed, upstream PR not yet opened.**
  Both give-up paths now return the same typed errors the non-retried paths
  return, surfaced through `%w`. The messages are carried on the typed errors
  rather than rebuilt around them, so both error strings stay byte-identical
  &mdash; the change is non-breaking for a caller matching on text and newly
  correct for one using `errors.As`.
  - Fork branch: `fix/preserve-typed-errors-on-retry-exhaustion` (`59e7e77`),
    pushed to `rshade/controld-go`, proposed upstream as
    [`baptistecdr/controld-go#68`](https://github.com/baptistecdr/controld-go/pull/68)
    (open), so the fork holds nothing that is not simultaneously proposed
    upstream.
  - Downstream effect: the `strings.Contains` fallbacks in
    `internal/controld/errors.go`, their two vendor-message constants, and the
    doc comment describing the workaround are deleted.
    `TestMapErrorHandlesVendorRetryExhaustion` now proves `MapError`'s
    `errors.As` branches fire on a real exhausted retry, asserting the error
    code (`controld_rate_limited` vs `controld_upstream_unavailable`) rather
    than only the shared `ExitNetwork` exit code.

### 3.2 Per-Request Impersonation / Header Options

- **Current Behavior in v0.0.10:**
  Headers can only be configured client-wide during client construction via `controld.Headers(http.Header)`.
- **Proposed Upstream Fix:**
  Support functional request options (e.g. `api.ListDevices(ctx, params, controld.WithHeader("X-Force-Org-Id", orgID))`) or context-driven headers, allowing a single client instance to dispatch multi-tenant or impersonated calls without recreating the client.
