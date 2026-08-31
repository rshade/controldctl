# controldctl

A CLI and MCP server for the [ControlD](https://controld.com) DNS filtering
API, built on [ax-go](https://github.com/rshade/ax-go) and
[baptistecdr/controld-go](https://github.com/baptistecdr/controld-go).

## Install

```bash
go build -o bin/controldctl .
```

## Authenticate

Set `CONTROLD_API_TOKEN`, or pass `--api-token`, or point `--config` at a
Hujson file with an `api_token` field. Precedence is `--api-token` >
`CONTROLD_API_TOKEN` > `--config`.

## Usage

```bash
controldctl devices list --format=json
controldctl profiles list --format=json
controldctl profiles filters list --profile-id=<id>
controldctl profiles rules create --profile-id=<id> \
  --hostnames=ads.example.com --do=0
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
`[y/N]` confirmation prompt when running in a terminal.

## Command tree

- `devices` - manage ControlD devices (endpoints)
  - `list`, `create`, `update`, `delete`, `types`
- `profiles` - manage ControlD profiles
  - `list`, `create`, `update`, `delete`
  - `options` - profile-level options: `list`, `update`
  - `filters` - category filters: `list`, `update`
  - `services` - service (app/site) rules: `list`, `update`
  - `rules` - custom domain rules: `list`, `create`, `update`, `delete`
  - `folders` - custom-rule folders: `list`, `create`, `update`, `delete`
- `mcp-server` - run as an MCP server (stdio or HTTP transport)
- `--mcp` - shorthand for `mcp-server`
- `__schema` - emit the machine-discoverability schema (`ax` or `mcp` format)

Run `controldctl [command] --help` or `controldctl __schema` for the full,
authoritative set of flags per command.

Out of scope for this CLI: Users/account info, Billing, Network Stats,
Access/IP logs, org impersonation, and mass provisioning.

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
make build   # go build -o bin/controldctl .
make test    # go test ./...
make lint    # go vet ./... && gofmt -l .
make fmt     # gofmt -w .
```
