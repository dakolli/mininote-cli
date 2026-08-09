# mininote CLI

A typed command-line client for the mininote RPC API, generated directly from
the API catalog (`../intro.json`). It wraps a fully generated Go client with a
Cobra command tree, so every RPC in the spec is exposed as a command.

## Layout

```
cli/
├── gen/               # generator: intro.json → typed Go client
│   └── spec/          # shared spec loader/normalizer (used by both generators)
├── client/            # package client — the generated HTTP client
│   ├── client.go      # hand-written runtime (envelopes, auth, APIError, guards)
│   ├── types.gen.go   # GENERATED — 308 request/response structs
│   └── methods.gen.go # GENERATED — 202 typed RPC methods
└── cmd/
    ├── cmdgen/        # generator: intro.json → cobra command tree
    ├── commands.gen.go# GENERATED — 25 service commands + 202 method subcommands
    ├── root.go        # root command, flags, config resolution
    ├── auth.go        # login / logout / whoami
    └── config.go      # session config persistence
```

Everything under `*.gen.go` is machine-generated and must not be edited by
hand. Edit the generators (`gen/`, `cmd/cmdgen/`) instead, then regenerate.

## Build & regenerate

```sh
cd cli
go install mininote.dev/cli/cmd/mininote   # installs `mininote` to $GOPATH/bin
go generate ./...     # regenerates client/*.gen.go and cmd/commands.gen.go
```

The entry point lives at `cmd/mininote/main.go` so `go install` produces a
`mininote` binary (the module root is a library of packages).

## Quick start

```sh
# Authenticate with an API key (folder-scoped). The key is picked up
# automatically from MININOTE_RPC_KEY / MININOTE_TOKEN, or pass --token.
export MININOTE_RPC_KEY="mnk_..."

mininote page tree                          # list your pages
mininote page get --id p_abc123             # fetch one page
mininote search query --query "notes"       # full-text search
mininote page upsert --path docs/readme --body '# Hello'   # create/update a page
```

The equivalent raw `curl` for the last command:

```sh
curl -X POST https://mininote.ink/rpc/Page/upsert \
  -H "Authorization: Bearer $MININOTE_RPC_KEY" \
  -H "Content-Type: application/json" \
  --data '{"args":{"path":"docs/readme","body":"# Hello"}}'
```

## Configuration

Authentication and endpoint resolution (highest precedence first):

| Setting    | Source |
|------------|--------|
| `--base-url` | flag |
| `MININOTE_BASE_URL` | environment |
| config file `baseURL` | `cli.json` |
| default | `https://mininote.ink` |
| `--token` | flag |
| config file `token` | `cli.json` |
| `MININOTE_RPC_KEY` / `MININOTE_TOKEN` | environment |

Global flags:

- `--base-url` — server base URL (default `https://mininote.ink`)
- `--token` — auth token/key (overrides stored config and env)
- `--config` — config file path (default `$XDG_CONFIG_HOME/mininote/cli.json`
  or `~/.config/mininote/cli.json`)
- `--compact` — single-line JSON output instead of pretty-printed

The config file is written with `0600` permissions and stores the session
after `mininote login`.

## Auth modes

- **API key** (`mnk_...`) — folder-scoped key for the key-reachable API. Passed
  as a bearer token. This is what the MCP/agent integration uses.
- **Session token** — obtained via `mininote login --handle X --password Y`
  (session-only endpoints) and stored in the config file.

The client auto-detects keys by the `mnk_` prefix. **Control-plane endpoints
are never callable with an API key** — the client rejects them up front:

- Auth: login, loginVerify, register, logout, refresh, me, changePassword,
  deleteAccount, requestPasswordReset, resetPassword
- 2FA: setupTwoFactor, confirmTwoFactor, disableTwoFactor, twoFactorStatus,
  regenerateRecoveryCodes
- Sessions/devices: listSessions, revokeSession, revokeOtherSessions,
  listTrustedDevices, revokeTrustedDevice
- Key management: createAPIKey, listAPIKeys, revokeAPIKey, rotateAPIKey

Attempts return `Error: this endpoint is not available to API keys (status 403)`
without any request hitting the network.

## Commands

`mininote <service> <method> [flags]` — one service command per RPC service,
one subcommand per method. Method flags map 1:1 to the request JSON fields:

```sh
mininote page get --id p_abc
mininote tag create --name bug --color red
mininote page update --id p_abc --title "New title" --body "…"
```

Flag types follow the schema: `--flag value` for strings/numbers/bools,
comma-separated lists for arrays (`--ids a,b,c`), and a JSON string for
freeform/map or nested-object params (`--config '{"width":"full"}'`). Required
params are enforced by Cobra.

Top-level commands:

- `version` — print the CLI version
- `login` — log in with a session and store the token
- `logout` — clear the stored token
- `whoami` — show the current session identity (`Auth.me`; session only)
- `completion` — shell autocompletion (Cobra built-in)

## The generated client

`client` is importable on its own:

```go
c, err := client.New("https://mininote.ink", client.WithAPIKey(key))
res, err := c.PageTree(ctx)                                   // void request
page, err := c.PageGet(ctx, client.PageGetParams{ID: "p_1"})  // typed params
```

- Request/response envelopes `{"args": …}` / `{"data": …}` are handled by the
  runtime.
- Non-2xx responses become `*client.APIError{StatusCode, Message, Code, Body}`.
- Optional schema fields are pointers (`*string`, `*bool`, …); slices and maps
  stay nilable.
- `number` maps to `float64`; freeform objects map to `map[string]any`.

## Testing

```sh
go test ./...    # httptest smoke tests for the client runtime + session-only guard
```

The smoke tests cover the `{"args":…}`/`{"data":…}` envelopes, typed decoding,
API error parsing, and the API-key control-plane guard.
