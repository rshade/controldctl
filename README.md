# controldctl

A CLI and MCP server for the [ControlD](https://controld.com) DNS filtering
API, built on [ax-go](https://github.com/rshade/ax-go) and
[baptistecdr/controld-go](https://github.com/baptistecdr/controld-go).

## Install

```bash
go build -o bin/controldctl ./cmd/controldctl
```

Or download a release binary from the
[Releases](https://github.com/rshade/controldctl/releases) page.

**`go install` will not work right now.** `go.mod` carries a `replace`
directive pointing `github.com/baptistecdr/controld-go` at a commit of
[`rshade/controld-go`](https://github.com/rshade/controld-go), and Go refuses
to `go install <pkg>@latest` any module whose `go.mod` contains a `replace`.
Build from a clone or use a release binary until then; both are unaffected.

The fork exists only to carry changes that are on their way upstream — it
preserves upstream's module path, so the directive is a single line and
nothing in the source refers to the fork. **The exit condition is upstream
merging the change**: once `baptistecdr/controld-go` ships a release
containing the typed-error fix, the `replace` line and this note are deleted
together and `go install` works again.

## Authenticate

Set `CONTROLD_API_TOKEN`, or pass `--api-token`, or point `--config` at a
Hujson file with an `api_token` field. Precedence is `--api-token` >
`CONTROLD_API_TOKEN` > `--config`.

To act as one of your organization's sub-organizations, pass `--org-id`, set
`CONTROLD_ORG_ID`, or add an `org_id` field to the same config file — the
same precedence applies: `--org-id` > `CONTROLD_ORG_ID` > `--config`. The
value is sent as ControlD's `X-Force-Org-Id` header, and every success
envelope reports the tenant it acted on in `meta.org_id` (empty means the
token's own account, and the key is never omitted). While an organization is
in effect, **every** mutation requires `--yes`, not just deletes — a
cross-tenant change lands on an account you do not own. Without `--org-id`,
nothing about the CLI's behavior changes.

## Usage

```bash
controldctl devices list --format=json
controldctl profiles list --format=json
controldctl profiles filters list --profile-id=<id>
controldctl profiles rules create --profile-id=<id> \
  --hostnames=ads.example.com --do=0
controldctl users show                 # the account behind the token
controldctl ip show                    # your egress IP, as ControlD sees it
controldctl network list               # ControlD's own PoP health
controldctl access list --device-id=<id>
controldctl billing payments           # payment history (financial data)
controldctl devices list --org-id=<sub-org id>
controldctl mcp-server                 # run as an MCP server over stdio
controldctl --mcp                      # same thing, shorter
```

Every command always writes the same JSON envelope, regardless of `--format`.
`--format` only controls confirmation-prompt behavior: `json` runs in machine
mode, where a confirmation-gated command (e.g. any `delete`) fails with a
`confirmation_required` error unless `--yes` is also passed; `human` runs in
interactive mode, prompting `[y/N]` instead. If `--format` is omitted,
`controldctl` picks a default based on whether stdout is a TTY.

Every mutating command supports `--dry-run`, which emits the response
envelope without making the underlying API call. Every `delete` command (and
any other confirmation-gated operation) requires `--yes`, or an interactive
`[y/N]` confirmation prompt when running in a terminal. Every command also
accepts `--idempotency-key`, an opaque key used to prevent duplicate-create
retries.

`profiles rules create` places new rules in the default rule folder (folder
`pk` `0`) unless you pass `--group=<folder pk>`. Run
`controldctl profiles folders list --profile-id=<id>` to see every folder's
`pk`, and list a folder's rules with
`controldctl profiles rules list --profile-id=<id> --folder-id=0`.

## Command tree

- `access` - the known-IP allowlist authorizing a device's resolver
  - `list`, `learn`, `delete`
- `billing` - the account's payments, subscriptions, and active products
  (financial data)
  - `payments`, `subscriptions`, `products`
- `devices` - manage ControlD devices (endpoints)
  - `list`, `create`, `update`, `delete`, `types`
- `profiles` - manage ControlD profiles
  - `list`, `create`, `update`, `delete`
  - `options` - profile-level options: `list`, `update`
  - `filters` - category filters: `list`, `update`
  - `services` - service (app/site) rules: `list`, `update`
  - `rules` - custom domain rules: `list`, `create`, `update`, `delete`
  - `folders` - custom-rule folders: `list`, `create`, `update`, `delete`
- `ip` - the caller's egress IP as ControlD observes it
  - `show`
- `network` - ControlD's own point-of-presence and resolver health
  - `list`
- `users` - the account record behind the configured token
  - `show`
- `mcp-server` - run as an MCP server (stdio or HTTP transport)
- `--mcp` - shorthand for `mcp-server`
- `__schema` - emit the machine-discoverability schema (`ax` or `mcp` format)

Run `controldctl [command] --help` or `controldctl __schema` for the full,
authoritative set of flags per command.

Out of scope for this CLI: local DNS resolution or proxying (that is
ControlD's own [`ctrld`](https://github.com/Control-D-Inc/ctrld)), any local
persistent state, and analytics-data commands — ControlD's `/stats` and
`/queries` paths appear only in community documentation, not its official
API reference.

## MCP server

```bash
# stdio transport
controldctl mcp-server

# HTTP transport, loopback only by default
controldctl mcp-server --transport=http --addr=127.0.0.1:8080
```

Every non-reserved command (all but `__schema`, `mcp-server`, and
`completion`) is exposed as an MCP tool; `tools/call` runs the command in
machine mode and returns its JSON payload.

## Development

```bash
make build   # go build -o bin/controldctl ./cmd/controldctl
make test    # go test ./...
make lint    # go vet ./... && gofmt -l .
make fmt     # gofmt -w .
```
