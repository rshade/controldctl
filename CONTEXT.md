# controldctl Context & Boundaries

## Core Architectural Identity

`controldctl` is a Go CLI — and, through the same command tree, a Model
Context Protocol (MCP) server — for managing [ControlD](https://controld.com)
DNS filtering resources: devices, profiles, filters, services, custom rules,
and rule folders, plus the account and organization surface — account info,
egress IP, PoP health, and the device known-IP allowlist — with
cross-tenant impersonation via `--org-id`. It's built on
[`ax-go`](https://github.com/rshade/ax-go) (a shared "Agentic Experience"
foundation that gives any Cobra CLI deterministic JSON envelopes, structured
errors, dry-run/confirmation safety primitives, and a free MCP server derived
from the same command tree) and
[`baptistecdr/controld-go`](https://github.com/baptistecdr/controld-go) (the
community ControlD API client). Its purpose: make ControlD's account-management
surface scriptable for humans and directly callable by LLM agents, with
identical behavior and identical safety guarantees either way.

## Technical Boundaries ("Hard No's")

- Does not implement DNS resolution or act as a DNS proxy/resolver — that's
  ControlD's own [`ctrld`](https://github.com/Control-D-Inc/ctrld) client's
  job, a different tool entirely.
- The account and organization surface — account info, egress IP, PoP health,
  Access/IP logs, billing, organizations, org impersonation, and mass
  provisioning — is **in scope**. It was deferred from the initial "Core DNS
  control" build pending a scoping pass; that pass is #6 and the decision is
  made, so this is no longer a boundary. What that surface may do is bounded
  by the rules below, not by a blanket exclusion: it is read-mostly, it holds
  no state, and its two sensitive edges carry explicit gates — provisioning
  codes are redacted unless a reveal flag is passed, and every mutation made
  under `--org-id` requires confirmation, not just deletes.
- Does not persist any state of its own — no local database, no cache. Every
  command is a stateless call through to the ControlD API.
- Does not implement its own HTTP client for the ControlD API. All API access
  goes through `baptistecdr/controld-go`, reached only via the
  `internal/controld` adapter package — no command file imports the vendor
  library directly.
- Does not hold or manage authentication for the MCP server transport.
  `ax-go`'s `mcp-server` binds loopback-only by default and holds no
  credentials; putting authn/authz in front of a non-loopback exposure is the
  operator's responsibility.

## Data Source of Truth

The live ControlD API (`api.controld.com`) is authoritative for all resource
state. `controldctl` never caches or diverges from it — every `list`/read
command reflects the API's response at call time.

## Interaction Model

- **Inbound:** the `controldctl` binary directly (human or scripted use), or
  `controldctl mcp-server` / `controldctl --mcp` (MCP client / LLM agent use).
  Both drive the identical Cobra command tree, so behavior never differs
  between the two.
- **Outbound:** HTTPS to `api.controld.com` via `baptistecdr/controld-go`,
  authenticated with a Bearer token resolved from `--api-token` >
  `CONTROLD_API_TOKEN` > a Hujson config file's `api_token` field.

## Verification

A proposed feature crosses these boundaries if it would: (a) implement local
DNS resolution or proxying, (b) add analytics-data commands (`/stats`,
`/queries`) — the deferral that survived #6's scoping pass, because those
paths appear only in community documentation and not in ControlD's official
reference, (c) add local persistent state, or (d) call the vendor library or
a raw HTTP client from a command file instead of going through
`internal/controld`.
