# mininote client/cli gen

A typed command-line client for the mininote RPC API. The whole client — typed
request/response structs, RPC methods, and the Cobra command tree — is generated
from the API catalog, so every endpoint in the spec is exposed as a
`mininote <service> <method>` command.

The catalog is re-captured live (`POST https://mininote.ink/rpc/_introspect`)
into [`intro.json`](intro.json) on **every** generation, so the client never
goes stale against the API — the committed file is only an offline fallback. By
default the generated surface is **pruned to what RPC API keys can call** (see
[Key-available surface](#key-available-surface)); `-full` restores everything
the spec publishes.

## Prerequisites

- Go 1.26+
- Network access to `https://mininote.ink` for generation (live spec capture)
- [`intro.json`](intro.json) — the API catalog, committed at the repo root but
  re-captured live on every `go generate` (a stale offline fallback only)

## Repository layout — read this first

The repo has **two roots**, and this asymmetry trips people up:

```
mininote-gen/
├── intro.json               # spec: the API catalog (re-captured live every generate; committed as an offline fallback)
├── api-key-forbidden.txt    # data: Service.method routes pruned by default (see "Key-available surface")
├── .gitignore               # ignores **/*.gen.go (generated files are never committed)
├── README.md
└── cli/                     # module root: go.mod (module github.com/dakolli/mininote-cli) + all Go code
    ├── client/              #   runtime + types.gen.go + methods.gen.go
    ├── cmd/                 #   cobra tree (commands.gen.go) + hand-written plumbing
    └── gen/                 #   the generators
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

## Develop

Build and regenerate from the source repo (`mininote-gen/`) on your machine:

```sh
cd mininote-gen/cli        # local module root inside the workspace go.work
go generate ./...          # live-captures the spec and rebuilds the .gen.go files
go build ./cmd/mininote    # or: go install ./cmd/mininote
```

For deterministic, network-free regeneration (CI-friendly) use the offline
mode instead: `go run ./gen -offline` + `go run ./cmd/cmdgen -offline` — no
live capture, no `STALE SPEC` warning. The published mirror is assembled from
this repo by `scripts/publish.sh`; see its header for the whitelist.

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

**Auth modes.** API keys (`mnk_...`) work for content routes only. The
generated client is strictly key-available: the session-only control plane
(login, 2FA, sessions, key management) is redacted from the live introspect
route and pruned out, so the CLI ships no `auth` commands at all — there is no
`mininote login` / `mininote whoami`. The hand-written `sessionOnly` fast-fail
map in the runtime still guards the `-full` builds.

## How it's generated

```
POST /rpc/_introspect  ──▶  intro.json  (re-captured on every go generate)
   │  (gen/spec normalizes the spec, then prunes to the key-available surface)
   ▼
gen/          ──text/template──▶  client/types.gen.go   (all types — never pruned)
                                 client/methods.gen.go  (key-available methods only by default)
cmd/cmdgen/   ──text/template──▶  cmd/commands.gen.go   (services with ≥1 surviving method)
```

Every `go generate` first re-captures the catalog from
`POST https://mininote.ink/rpc/_introspect` (bearer auth from
`MININOTE_ADMIN_AGENT_KEY`, falling back to `MININOTE_RPC_KEY`, then
`MININOTE_TOKEN`) and **rewrites the repo-root `intro.json`**, then builds the
model from that fresh file — the committed snapshot is never consumed stale.
Passing `-in <file>` to `gen`/`cmdgen` explicitly uses that file instead and
prints a loud `STALE SPEC` warning (offline builds only).

**Deterministic offline regeneration (CI):** pass `-offline` to skip the live
capture entirely and regenerate silently from the committed `intro.json` — no
network, no `STALE SPEC` warning. Use `go run ./gen -offline` +
`go run ./cmd/cmdgen -offline` in CI so the staleness gate
(`go generate ./... && git diff --exit-code`) is deterministic; local
`go generate ./...` keeps live capture. `-in` and `-offline` are mutually
exclusive.

The generators only emit types and method stubs; the runtime
(`client/client.go`) is hand-written and handles the `{"args":…}` / `{"data":…}`
envelopes, auth, and error decoding. Regenerate from inside `cli/` with
`go generate ./...` — the output is deterministic and gofmt'd.

## Key-available surface

RPC API keys (`mnk_...`) cannot call the whole catalog: the server answers
`403 FORBIDDEN this endpoint is not available to API keys` for ~77 control-plane
routes, `Auth/*` is session-only, and `Admin/*` is control-plane. The generated
client and CLI therefore **prune that surface by default**, exposing only what a
bare API key can actually call. The session-only fast-fail runtime map
(`sessionOnly` in `client/client.go`) is unchanged and still guards the `-full`
builds.

- **The prune list is data, not code:** [`api-key-forbidden.txt`](api-key-forbidden.txt)
  at the repo root, one `Service.method` per line (`#` comments and blank lines
  ignored; methods are mixed case, e.g. `Page.listPrefix`). It is seeded from
  the union of the server's 403 capture, the `Auth.*` session-only set, and all
  `Admin.*` methods. Change the file and regenerate to change the surface.
- **Every generation prints the prune summary** — `kept N/M methods (pruned P);
  S services fully removed: …` — plus a loud (non-fatal) warning listing any
  forbidden entries that matched **no** method in the current spec (stale
  entries). Expected-stale entries are the `Admin.*` and session-only `Auth.*`
  routes: the introspection route redacts the control plane, so those never
  appear in a live spec. Anything beyond them means the spec or the list
  drifted and deserves a look.
- **`-full` escape hatch:** `go run ./gen -full` / `go run ./cmd/cmdgen -full`
  (or pass no forbidden file) skips pruning and generates the complete surface
  the introspect route publishes — for session-token users who can call
  everything.
- **Types are never pruned** — all spec types stay in `types.gen.go` (types are
  not RPCs and cannot 403).

### Refreshing the forbidden list

The server's ACL changes; when the prune numbers drift, re-probe **against a
throwaway/disposable workspace** with an API key, never real data:

1. Make sure the catalog is current (a plain `go generate ./...` re-captures it
   into the repo-root `intro.json`, which lists every route).
2. POST `{"args":{}}` to every published route with the throwaway key; any
   route answering `403 FORBIDDEN … not available to API keys` belongs in
   `api-key-forbidden.txt` (plus the `Auth.*` and `Admin.*` sets).
3. Update the file, regenerate, and confirm the prune summary moved the way the
   probe said it should.

Never probe on real data (see AGENTS.md Gotchas — `ticket set` with no flags is
destructive).

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
