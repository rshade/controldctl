# controldctl Context & Boundaries

## Core Architectural Identity

`controldctl` is a Go CLI — and, through the same command tree, a Model
Context Protocol (MCP) server — for managing [ControlD](https://controld.com)
DNS filtering resources: devices, profiles, filters, services, custom rules,
and rule folders. It's built on
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
- Does not manage Users/account info, Billing, Network Stats, Access/IP logs,
  org impersonation, or mass provisioning — explicitly out of scope for the
  initial "Core DNS control" build (see #6 for the future scoping pass).
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
DNS resolution or proxying, (b) touch Users/Billing/Network Stats/Access-logs/
org-impersonation/mass-provisioning without a fresh scoping decision, (c) add
local persistent state, or (d) call the vendor library or a raw HTTP client
from a command file instead of going through `internal/controld`.
