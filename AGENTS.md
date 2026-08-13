# AGENTS.md — mininote-gen

You are working on `mininote-gen`, a Go repo that turns the mininote RPC API's introspection
catalog (`intro.json`) into a **typed client library** (`github.com/dakolli/mininote-cli/client`) and a
**full-surface CLI** (`mininote`). Think as a principal-level Go engineer who ships code
generators: the generated code is deterministic, gofmt-clean, and never hand-edited — the
spec is the source of truth, and the runtime that executes it is small, hand-written, and
deliberately boring.

## What this repo is

- **A generator pipeline + a thin runtime.** `intro.json` (committed at the repo root, **re-captured
  live from `mininote/rpc/_introspect` on every `go generate`**) is the API catalog for the server's
  `introspect` plugin. A live capture carries ~24 services / ~155 methods / ~256 named types (the route
  redacts the control plane: no `Admin`, and `Auth` exposes only `me`), plus `cross_refs` and
  `boundary_warnings`. The generators read it and emit Go; a hand-written HTTP client provides the
  `{"args":…}`/`{"data":…}` envelope, auth, and error decoding that the generated code builds on.
- **Not the backend.** There is no server here. This is a *client* of the mininote.ink JSON-RPC
  backend, and everything is driven by what the spec says. When the backend changes, `intro.json`
  must be re-captured before regeneration.
- The module is `github.com/dakolli/mininote-cli`, nested one level down at `cli/` (the repo root holds
  `intro.json`, `.gitignore`, `README.md`). **The spec lives at the repo root; the module lives in
  `cli/`.** This asymmetry trips people up — see Gotchas.

## Codebase Orientation

| File | Role | Generated? |
|---|---|---|
| `intro.json` | API catalog (services/methods/types) | committed snapshot, **re-captured live on every `go generate`** |
| `api-key-forbidden.txt` | `Service.method` routes pruned from the generated surface by default (data, not code) | committed snapshot, manually refreshed |
| `cli/go.mod`, `cli/go.sum` | module `github.com/dakolli/mininote-cli`, Go 1.26.4, only dep: `spf13/cobra` | hand-written |
| `cli/gen/gen.go` | client generator entrypoint (`gen`) | hand-written |
| `cli/gen/spec/spec.go` | loads + normalizes `intro.json` into a Go-ready `Model` | hand-written |
| `cli/gen/templates/*.tmpl` | `text/template` skeletons for types/methods | hand-written |
| `cli/cmd/cmdgen/main.go` | cobra command-tree generator (`cmdgen`) | hand-written |
| `cli/client/client.go` | **runtime**: envelope, auth modes, `APIError` decoding | hand-written |
| `cli/client/types.gen.go` | all spec types (never pruned) | **generated, gitignored** |
| `cli/client/methods.gen.go` | RPC methods on `*Client` (key-available only by default; `-full` for everything) | **generated, gitignored** |
| `cli/cmd/commands.gen.go` | service commands × method subcommands (key-available services by default) | **generated, gitignored** |
| `cli/cmd/root.go`, `config.go`, `auth.go`, `main.go` | cobra wiring, config file, hand-written `logout` | hand-written |
| `cli/cmd/mininote/main.go` | `func main() { cmd.Execute() }` | hand-written |
| `cli/client/client_test.go` | httptest unit tests (no network) | hand-written |
| `cli/client/integration_test.go` | live tests, skipped unless `MININOTE_RPC_KEY` set | hand-written |

### Generated vs hand-written — the split that matters

- **Generated:** every typed struct, every RPC method stub, and the whole cobra tree. They are
  rebuildable from `intro.json` alone and are **gitignored** (`**/*.gen.go` plus explicit entries in
  the repo-root `.gitignore`). Never edit them — edit the templates or the spec and regenerate.
- **Hand-written:** the runtime (`client/client.go`), the spec normalizer (`gen/spec/spec.go`), the
  generators, and the CLI plumbing. This is where all real logic lives.
- The `//go:generate` directives are attached to the *hand-written* files they regenerate:
  `client/client.go#L18` and `cmd/root.go#L33` — both now run with
  `-introspect https://mininote.ink/rpc/_introspect -forbidden ../../api-key-forbidden.txt`, so a bare
  `go generate ./...` re-captures the spec live, rewrites the repo-root `intro.json`, and prunes the
  generated surface to the key-available routes (see the Gotchas entries below). Running
  `go generate ./...` from anywhere in the module rebuilds all three `.gen.go` files.

## Architecture: intro.json → text/template → .gen.go

```
intro.json  (live capture: POST /rpc/_introspect → rewritten every go generate)
   │  gen.LoadSpec → spec.Normalize → Model.PruneForbidden (api-key-forbidden.txt)
   ▼
gen/gen.go  ──types.gen.tmpl──▶  client/types.gen.go   (all types — never pruned)
   └────────methods.gen.tmpl──▶  client/methods.gen.go  (key-available methods by default)
cmd/cmdgen  ──commands.tmpl──▶  cmd/commands.gen.go    (services with ≥1 surviving method)
```

1. **`spec.LoadSpec` / `spec.Normalize`** (`cli/gen/spec/spec.go`): unmarshal `intro.json`, then
   normalize to a sorted, Go-ready `Model`. Key rules:
   - `GoName` (`spec.go#L100`) converts spec names to exported Go identifiers: splits on
     non-alphanumerics, upper-cases segments, and maps `id` → `ID` (e.g. `page_id` → `PageID`,
     `base_updated_at` → `BaseUpdatedAt`).
   - `FieldGoType` (`spec.go#L173`): scalars are pointer types when optional (`*string`), required
     scalars are plain; slices/maps are always non-pointer (nilable). **The generated structs are
     pointer-heavy on purpose** — a missing JSON key is `nil`, not `""`/`0`.
   - `responseTypeExpr` (`spec.go#L251`) maps `responseTypeName` → named struct, else
     `Record<string, boolean>` → `map[string]bool`, else `T[]` → `[]NestedType`.
   - `Normalize` (`spec.go#L274`) fails loudly on spec shapes it can't map (unrecognized response
     TS, params-without-request-type) — a spec change that breaks the generators fails at
     generation time, not at runtime.
2. **`gen/gen.go`** renders both client templates and gofmts the output (`render`, `gen.go#L86`).
   The templates are tiny (`types.gen.tmpl` = 13 lines, `methods.gen.tmpl` = 12 lines); nearly all
   intelligence lives in the spec model, not the templates.
3. **`cmd/cmdgen/main.go`** renders the cobra tree. Each service → `mininote <service>` command
   (`lowerFirst(Service)`: `Page` → `page`); each method → a subcommand with one flag per param.
   `paramKind` (`cmdgen/main.go#L207`) picks the pflag family: scalar kinds get typed flags
   (`--id string`, `--limit float`), `[]string` gets a `StringSlice` flag, and anything the
   generator can't type (structs, `map[string]any`) becomes a `--<name> string` flag marked `JSON`
   whose value is `json.Unmarshal`ed. `buildServices` (`cmdgen/main.go#L163`) preserves the model's
   sorted service order and attaches each service's title from the spec.
4. **Runtime.** `client.Client.do` (`client/client.go#L127`) POSTs `{"args": req}` to
   `baseURL + "/rpc/<Service>/<method>"`, unwraps `{"data": resp}`, and turns non-2xx into
   `*client.APIError`. That's the entire network layer. Every generated method is
   `func (c *Client) PageTree(ctx) (TreeResult, error)` delegating to `do`.

### The `{"args":…}` / `{"data":…}` envelope

- Requests are always wrapped: `{"args":{}}` even for void-parameter methods (see
  `client_test.go#L65` — `AuthMe` sends `{"args":{}}`). Params are flat key/value objects, never
  positional.
- Responses are unwrapped from `{"data": ...}`. `decodeData` (`client.go#L185`) falls back to
  decoding the bare body if the envelope key is absent (robust to bare responses).
- Errors: `decodeError` (`client.go#L201`) tries `{"error":{"message","code"}}`, then
  `{"error": string}`, then `{"message": string}`, then the raw body. Non-2xx → `*APIError` with
  `StatusCode`, `Message`, `Code`, `Body` (`client.go#L229`). The CLI prints one friendly
  `Error: <message> (status N)` line via `rpcErr` (`cmd/root.go#L114`).

### Auth modes

- **Session token:** `client.WithToken(t)` (`client.go#L94`). Sent as `Authorization: Bearer <t>`.
  Can call every RPC. Tokens come from `--token`, the env, or the config file — there is no
  hand-written `mininote login` to persist one (see below).
- **API key (`mnk_...`):** `client.WithAPIKey(k)` (`client.go#L104`). The CLI auto-detects the
  `mnk_` prefix in `getClient` (`cmd/root.go#L62`). Keys are **folder/workspace-scoped** and can
  only reach content routes.
  - **Client-side fast-fail:** the `sessionOnly` map (`client.go#L33-L58`) blocks the 25 `Auth/*`
    control-plane RPCs (login, 2FA, sessions, key minting) *before* any request — key minting keys
    is privilege escalation. This returns a synthetic `403 FORBIDDEN` without touching the network.
  - **Server-side enforcement is much broader.** ~77 more routes return
    `403 FORBIDDEN this endpoint is not available to API keys` from the server: the entire
    `Workspace` control surface (create/keys/invites/members/rename/delete/…), `Ticket.myWork`
    (the *only* Ticket method blocked — `Ticket.get/set/patch/changes/delete/restore/history` are
    key-reachable, verified live), `Page.
    resolveKey`, `Page.moveToWorkspace`, `Page.usage`, `Settings.*`, `Template.*`, `Srs.*`,
    `Share.*`, `Upload.*`, `Activity.*`, `Dashboard.*`, `Lock.*`, `Notifications.*`,
    `Presence.*`, `Profile.*`, `Realtime.*`. The current full list is captured in
    `~/.config/mininote/403routes.txt`. **Do not add routes to `sessionOnly` without verifying
    server behavior** — the client list exists only to fail fast on Auth; the server owns the real
    ACL and it is wider.
  - Practically, the key-reachable surface is: `Page` reads/writes (tree, get, listPrefix, create,
    update, upsert, delete, pathOf, refs, changes, clone, import, export, exportAll), `Search.
    query`, `Tag.list`, `Annotation.*`, `Comment.*`, `History.*`, `Ticket.*` (except
    `Ticket.myWork`), `Share.apiDocs`, `CalFeed.*`, `Event.*`. The current full forbidden list is
    captured in `~/.config/mininote/403routes.txt` — consult it rather than guessing.

### Config & flags (the CLI layer)

- Precedence: **flag > config file > env**. `--base-url` / `--token` override the config file
  (`cmd/root.go#L72-L92`); env fallbacks are `MININOTE_RPC_KEY`, then `MININOTE_TOKEN`; the base
  URL env is `MININOTE_BASE_URL`, defaulting to `https://mininote.ink`.
- `--compact` prints single-line JSON; default is `json.MarshalIndent` 2-space (`printResult`,
  `cmd/root.go#L131`). Every generated command accepts it via the root persistent flag.
- Hand-written top-level commands: `logout` (clears the stored token from the config file — it
  makes no RPC call) and `version`. There is **no** `mininote login` or `mininote whoami`, and no
  generated `auth` service: Auth RPCs are session-only and redacted from the live introspect route,
  and the generated client is strictly key-available by design, so no command can reach them.

## The CLI surface & what's in the responses

- The pruned surface is ~77 methods across the remaining services (exact counts vary per capture): one
  command per key-available method: `mininote <service> <method> [flags]`. Run
  `mininote --help` for the list, `mininote <service> --help` for methods, `mininote <service>
  <method> --help` for flags.
- Service/method name mapping is verbatim spec (`page listPrefix`, `search query`,
  `workspace forKey` — note the mixed case in method names; cobra uses them as-is).

### The human-facing id fields (verified live, 2026-08-11)

These matter to anyone building pickers/boards that want to show `SLUG-NUM` (e.g. `PLUG-9`). There
is **no `human_id` field anywhere in the RPC** (`grep -c human_id intro.json` → 0). The composite
`SLUG-NUM` must be built client-side from two pieces:

- **Prefix:** `Workspace.forKey` (void request) → `{workspaces:[{id, name, kind, role, color, key,
  created_at}], current}`. The `key` field **is** the slug prefix (verified: `"key":"PLUG"` for the
  PLUGIN workspace, `"key":"AGENT"` for the AGENTS workspace). One round trip, one call.
- **Number:** available from several endpoints, not just `Page.get`:
  - `Page.listPrefix {path?}` — **depth-1 directory listing** whose rows always carry `num`
    (verified with a workspace-scoped API key: `--path kanban` → 14/14 rows with `num`,
    `PLUG-9`→9, `PLUG-17`→17; root listing → 7/7). One round trip replaces the whole board.
  - `Page.tree` (void request, **no params**) — includes `num` per row for *some* callers and not
    others (verified: admin-tier key → 54/54 rows with `num`; the PLUGIN workspace key → 0/27).
    The gating is server-side and caller-dependent; treat `num` as best-effort from `tree`.
  - `Page.get {id}` — full `Page` with `num`, `slug`, `prev_slug` (and `id`, `parent_id`,
    `space_id`, …). Per-page (N+1) but complete.
  - `Search.query` hits — **never** have `num`/`slug`/`key`: the `Hit` type is fixed at
    `{id, title, snippet}` (types.gen.go#L751). `--kinds` (e.g. `ticket`) works; `--scope_root`
    is accepted but **silently ignored** by the server (verified: identical result sets with and
    without it, including out-of-scope pages). Snippets contain `<b>` highlight tags and HTML
    entities — strip before display.
- `Page.resolveKey {key}` resolves `"PLUG-9"` → `{id}` but is **403 for API keys** (session-only,
  server-side). It's also the wrong direction for pickers (key→id; pickers need id→key).

**Composing `SLUG-NUM` (cheapest recipe):** `Workspace.forKey` (1 round trip, gives prefix) +
`Page.listPrefix {path: <root>}` (1 round trip, gives `num` for every row) = **2 round trips, no
N+1**, for a board/folder picker. `Page.get` per row is only needed when `listPrefix` isn't
applicable (e.g. arbitrary page ids from search hits).

## Build / test / development workflow

```sh
# Regenerate (from anywhere in the module):
go generate ./...          # rebuilds client/types.gen.go, client/methods.gen.go, cmd/commands.gen.go

# Build / run / install:
go build ./cmd/mininote
go run ./cmd/mininote --help
go install ./cmd/mininote  # installs `mininote` to $GOPATH/bin

# Test:
go test ./...              # httptest unit tests, no network
go test -run TestIntegration -v ./client/   # live; SKIPPED unless MININOTE_RPC_KEY is set
```

- `go generate` runs each `//go:generate` line in its **file's directory** (client/ and cmd/), so
  the relative `-forbidden ../../api-key-forbidden.txt` always resolves to the repo root — that's
  why the directives work from anywhere in the module. The spec itself is captured live; the repo-root
  `intro.json` is rewritten every run (auth from `MININOTE_ADMIN_AGENT_KEY`, then `MININOTE_RPC_KEY`,
  then `MININOTE_TOKEN`).
- **CI / offline regeneration:** use `go run ./gen -offline` + `go run ./cmd/cmdgen -offline` for a
  deterministic, network-free rebuild from the committed `intro.json` (no `STALE SPEC` warning — that
  banner is only for explicit `-in`). Local `go generate ./...` stays live-capturing; `-in` and
  `-offline` are mutually exclusive.
- The unit tests (`client_test.go`) spin up an `httptest.Server` and assert the exact request body
  (`{"args":{...}}`), the `Bearer` header, envelope decoding, and that the client-side
  session-only block returns 403 without a network call (`TestSessionOnlyBlockedForAPIKey`).
- Integration tests (`integration_test.go`) hit the real server when `MININOTE_RPC_KEY` is set.
  `TestIntegrationPageWriteRoundTrip` creates and deletes a page (cleanup via `t.Cleanup`), so it
  is safe to run, but it does touch real state — don't run with a production key you don't own.
- **Workflow when the API changes:** `go generate ./...` re-captures `intro.json` from the server's
  `/_introspect` route automatically, prunes to the key-available surface, and prints the prune summary
  (watch for stale-entry warnings and count drift — that is the drift alarm). Then `go test ./...`, then
  `go install`. Generated output is deterministic and gofmt'd, so diffs in `.gen.go` files are
  meaningful (though they're gitignored, so review the template/spec changes instead).

## Conventions

1. **The spec is the contract.** If a type or method behaves oddly, check `intro.json` first; the
   generators are a faithful 1:1 translation. Fix shape problems in `spec.Normalize`, not in the
   templates, unless it's genuinely a rendering concern.
2. **Never hand-edit `.gen.go`.** If you need a field the spec doesn't declare, either the spec
   snapshot is stale (re-capture) or the change belongs in the runtime, not the generated file.
3. **Keep the runtime boring.** `client.go` is the single place where envelope/auth/error logic
   lives. New hand-written code should build on `Client.do`, not reimplement HTTP.
4. **Pointer-heavy generated types:** always nil-check before dereferencing. `mininote-tools`
   ships a `deref` helper precisely for this — copy the pattern.
5. **Context first:** propagate `cmd.Context()` from cobra `RunE` into every client call. Don't
   use `context.Background()` in command handlers.
6. **Wrap errors:** `fmt.Errorf("page tree: %w", err)`; the CLI reports `*client.APIError` as a
   single friendly line (`rpcErr`) and anything else via `Execute` — don't double-print.
7. **Thin `RunE`s:** generated commands are already thin (build params → call → print). Keep
   hand-written commands (see `auth.go`) in the same shape.

## Gotchas

- **`ticket set` with no field flags is DESTRUCTIVE.** `Ticket.set`/`Ticket.patch` treat absent
  fields as *clear*, not *leave alone*: calling `mininote ticket set --node_id <id>` with no other
  flags wipes `owner`, `status`, `priority`, `type`, `due`, `estimate` on the server (verified
  live 2026-08-11 — PLUG-9 was clobbered and manually restored). Optional-field flags only take
  effect when passed (`cmd.Flags().Changed`), and an empty invocation sends an empty patch that
  the backend applies as a full overwrite. Never probe ticket mutations on real data.
- **`Ticket.set`/`Ticket.patch` are key-reachable.** Only `Ticket.myWork` is 403 for API keys.
  A workspace key can read and rewrite ticket metadata directly — `Page.upsert` is not the only
  mutation path.
- **Spec and module live in different roots.** `intro.json` sits at the repo root; `go.mod` sits
  in `cli/`. Generators resolve the module root by walking up for `go.mod` (`gen.go#L68`), and
  cmdgen recomputes its defaults from that root (`cmdgen/main.go#L42-L51`), so both work when
  invoked manually too — but a spec path passed explicitly is relative to cwd.
- **`page tree` takes no params.** There is no flag to request `num`; whether rows carry `num` is
  decided server-side per caller. If a picker needs reliable `num`, use `page listPrefix`, not
  `page tree`. (Verified 2026-08-11: workspace key tree → 0/27 with `num`; admin-tier key tree →
  54/54. Same endpoints, different caller.)
- **`search query --scope_root` is a no-op.** It parses and sends, but the server ignores it
  (identical results with/without). Don't build search scoping on it; test again after backend
  changes.
- **`Search.query` hits are display-hostile:** HTML `<b>` tags + entities in `snippet`, only
  `id`/`title`/`snippet` available. The plugin strips tags before rendering.
- **Client-side 403 ≠ server-side 403.** The `sessionOnly` list covers only `Auth/*`. Nearly every
  other control-plane route is forbidden by the *server* for API keys — a request will be sent and
  bounce with 403. This is by design; the client list exists to fail fast on auth-only routes.
- **`mnk_` prefix detection is the API-key switch.** A token that starts with `mnk_` puts the
  client in API-key mode (`cmd/root.go#L62`). Don't create keys with that prefix for sessions.
- **`intro.json` is re-captured live on every generation — never trust the committed copy.** The `//go:generate`
  directives run `gen`/`cmdgen` with `-introspect https://mininote.ink/rpc/_introspect`, which POSTs `{}`
  (bearer auth from `MININOTE_ADMIN_AGENT_KEY`, then `MININOTE_RPC_KEY`, then `MININOTE_TOKEN`) and rewrites
  the repo-root `intro.json` before building the model. The committed file is a fallback **only** for explicit
  `-in <file>` use / offline builds, and prints a loud `STALE SPEC` warning then. The introspection route
  redacts the control plane (no `Admin` service; `Auth` exposes only `me`), so a fresh capture has fewer
  methods than the old full 202-method snapshots.
- **`api-key-forbidden.txt` is a snapshot, refreshed manually.** It lists the `Service.method` routes the
  generated client/CLI prune by default (API keys cannot call them: server 403s, session-only `Auth/*`, and
  all `Admin/*`). Every generation prints a prune summary (`kept N/M methods (pruned P); S services fully
  removed`) plus a loud warning for stale entries (forbidden routes matching no method in the current spec).
  `Admin.*` and session-only `Auth.*` entries are *expected-stale* (the introspect route redacts them); any
  other stale entry means spec or list drift. Refresh by re-probing **against a throwaway workspace** with an
  API key (`POST {"args":{}}` to each route; 403 = key-blocked), never real data — see README
  "Key-available surface". `gen`/`cmdgen -full` skips the prune entirely.
- **Empty `cross_refs`:** the `cross_refs` top-level key exists but is `{}` — the generators ignore
  it. Don't rely on it for anything yet.
- **`boundary_warnings`:** one entry — `"type "Page" is returned by 2 services (History, Page) —
  data ownership is ambiguous"`. Relevant when `History.*` results and `Page.*` results are both
  decoded into `Page` structs in the same flow.
- **Go 1.26.4.** New toolchain; don't downgrade `go.mod`. Only runtime dep is cobra; `go.sum`
  stays tiny.
- **No `--format`, no interactive mode, no subcommand aliases.** The CLI is a 1:1 RPC surface.
  Opinionated workflow tooling belongs in `mininote-tools`, not here.

## Relationship to the rest of the workspace

- **mininote-tools** (Go CLI): consumes this module as a library — `import
  "github.com/dakolli/mininote-cli/client"`. It resolves through the workspace `go.work` (which lists
  `./mininote-gen/cli` and `./mininote-tools`); `mininote-tools/go.mod` has **no `require` lines**,
  so it only builds inside the workspace. It uses `PageTree` + `PageGet` today and lists
  `WorkspaceForKey` / `PageListPrefix` / `PageUpsert` as planned. If you change generated client
  APIs or the runtime, rebuild mininote-tools from the workspace root to confirm.
- **mininote.nvim** (Lua plugin): **does not** integrate this Go client. It learns RPC usage from
  this package's contracts (`/rpc/<Service>/<method>`, the envelope, the response shapes in
  `types.gen.go`) but implements its own async `curl` boundary in Lua. Keep the RPC surface here
  faithful to the server so the Lua plugin can keep tracking it by eye — especially the
  human-id/`num`/`key` findings above, which the plugin's pickers depend on.
- **This repo is a git repo** (4 commits); mininote-tools is not. `.env` at the workspace root
  holds `MININOTE_WORKSPACE_KEY` / `MININOTE_ADMIN_AGENT_KEY` — never print or commit them.
