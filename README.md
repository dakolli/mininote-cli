# mininote CLI

A typed command-line client for the mininote RPC API. The whole client — typed
request/response structs, RPC methods, and the Cobra command tree — is generated
from the API catalog in [`intro.json`](intro.json), so every endpoint in the spec
is exposed as a `mininote <service> <method>` command.

## Prerequisites

- Go 1.26+
- [`intro.json`](intro.json) — the API catalog, committed at the repo root
  (the generators read it; generated `*.gen.go` files are gitignored)

## Build & install

```sh
git clone <this-repo> && cd <repo>/cli

go generate ./...          # rebuilds client/*.gen.go and cmd/commands.gen.go
go install ./cmd/mininote  # installs `mininote` to $GOPATH/bin
```

Or without installing: `go run ./cmd/mininote --help`.

## Quick start

```sh
export MININOTE_RPC_KEY="mnk_..."     # folder-scoped API key

mininote page tree                              # list pages
mininote page get --id p_abc123                 # fetch one page
mininote search query --query notes             # full-text search
mininote page upsert --path docs/readme --body '# Hello'   # create/update
```

The equivalent raw call is:

```sh
curl -X POST https://mininote.ink/rpc/Page/upsert \
  -H "Authorization: Bearer $MININOTE_RPC_KEY" \
  -H "Content-Type: application/json" \
  --data '{"args":{"path":"docs/readme","body":"# Hello"}}'
```

## Configuration

- Endpoint: `--base-url`, `MININOTE_BASE_URL`, or config file (default
  `https://mininote.ink`)
- Auth: `--token`, config file, or env `MININOTE_RPC_KEY` / `MININOTE_TOKEN`
- Config file: `$XDG_CONFIG_HOME/mininote/cli.json` (0600 perms)
- `--compact` for single-line JSON output

**Auth modes.** API keys (`mnk_...`) work for content routes only. Control-plane
endpoints (login, 2FA, sessions, key management) are session-only and are
rejected client-side when using a key:

```sh
$ mininote auth me
Error: this endpoint is not available to API keys (status 403)
```

## How it's generated

```
intro.json
   │  (gen/spec normalizes the spec)
   ▼
gen/          ──text/template──▶  client/types.gen.go  (308 structs)
                                 client/methods.gen.go (202 RPC methods)
cmd/cmdgen/   ──text/template──▶  cmd/commands.gen.go  (25 services × methods)
```

The generators only emit types and method stubs; the runtime
(`client/client.go`) is hand-written and handles the `{"args":…}` / `{"data":…}`
envelopes, auth, and error decoding. Regenerate with `go generate ./...` — the
output is deterministic and gofmt'd.

## Tests

```sh
go test ./...        # unit tests (httptest, no network)
```

Live integration tests hit a real server and are skipped unless
`MININOTE_RPC_KEY` is set:

```sh
go test -run TestIntegration -v ./client/
```
