# mininote client/cli gen

A typed command-line client for the mininote RPC API. The whole client — typed
request/response structs, RPC methods, and the Cobra command tree — is generated
from the API catalog in [`intro.json`](intro.json), so every endpoint in the spec
is exposed as a `mininote <service> <method>` command.

## Prerequisites

- Go 1.26+
- [`intro.json`](intro.json) — the API catalog, committed at the repo root
  (the generators read it; generated `*.gen.go` files are gitignored)

## Repository layout — read this first

The repo has **two roots**, and this asymmetry trips people up:

```
mininote-gen/
├── intro.json        # spec root: the API catalog (committed, hand-captured)
├── .gitignore        # ignores **/*.gen.go (generated files are never committed)
├── README.md
└── cli/              # module root: go.mod (module mininote.dev/cli) + all Go code
    ├── client/       #   runtime + types.gen.go + methods.gen.go
    ├── cmd/          #   cobra tree (commands.gen.go) + hand-written plumbing
    └── gen/          #   the generators
```

Every `go` command (`go generate`, `go build`, `go test`, `go install`,
`go run`) must be run **from inside `cli/`** — the repo root has no `go.mod`,
so Go cannot resolve the module from there (`go generate ./...` at the repo
root fails with "directory prefix . does not contain modules listed in
go.work or their selected dependencies"). The `//go:generate` directives run
in their *file's* directory (`cli/client/`, `cli/cmd/`), which is why the
relative spec path `../../intro.json` always resolves to the repo root from
anywhere in the module.

When this repo is used inside the `workspace_mininote` `go.work` (which lists
`./mininote-gen/cli` and `./mininote-tools`), the same rule applies: run `go`
commands from `cli/` — the repo root is still not a module.

## Build & install

```sh
git clone https://github.com/dakolli/mininote-gen
cd mininote-gen/cli      # the Go module lives here, not at the repo root

go generate ./...          # rebuilds client/types.gen.go, client/methods.gen.go, cmd/commands.gen.go
go install ./cmd/mininote  # installs `mininote` to $GOPATH/bin
```

Or without installing: `go run ./cmd/mininote --help`. (All commands below
assume you're inside `cli/`.)

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
envelopes, auth, and error decoding. Regenerate from inside `cli/` with
`go generate ./...` — the output is deterministic and gofmt'd.

## Tests

```sh
# from cli/
go test ./...        # unit tests (httptest, no network)
```

Live integration tests hit a real server and are skipped unless
`MININOTE_RPC_KEY` is set:

```sh
go test -run TestIntegration -v ./client/
```
