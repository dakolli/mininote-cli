# mininote

A typed command-line client for the mininote.ink note-taking API.

Every API endpoint is exposed as a `mininote <service> <method>` command, with
one flag per parameter. Run `mininote --help` for the full list.

## Install

**Linux / macOS** — quick script (installs to `/usr/local/bin`, or `~/bin`
if it isn't writable):

```sh
curl -sfL https://raw.githubusercontent.com/dakolli/mininote-cli/main/install.sh | sh
```

**From source** — requires Go 1.26+:

```sh
go install github.com/dakolli/mininote-cli@latest
```

**Prebuilt binaries** for linux/darwin/windows (amd64 + arm64) are published on
[GitHub Releases](https://github.com/dakolli/mininote-cli/releases).

## Quick start

You need an API key (`mnk_...`) — mininote workspaces provide them. Point the
CLI at the server and it sends the key with every request:

```sh
export MININOTE_RPC_KEY="mnk_..."

mininote page tree                                # list pages
mininote page get --id p_abc123                   # fetch one page
mininote search query --query notes               # full-text search
mininote page upsert --path docs/readme --body '# Hello'   # create or update
mininote workspace forKey                         # your workspaces
```

Every command prints the response as indented JSON; add `--compact` for
single-line output.

## Configuration

| Setting | How |
|---|---|
| Server | `--base-url` (default `https://mininote.ink`), or `MININOTE_BASE_URL` |
| Auth | `--token` flag, config file, or env `MININOTE_RPC_KEY` / `MININOTE_TOKEN` |
| Config file | `~/.config/mininote/cli.json` (flags win over the file) |
| Output | `--compact` for single-line JSON |

Flags take precedence over the config file, which takes precedence over
environment variables.

## Commands

- `annotation` — read and manage inline annotations on notes
- `calFeed` — calendar feeds (iCal)
- `comment` — read and manage comments on notes
- `event` — calendar events
- `export` — export pages and documents
- `history` — page revision history
- `page` — read, create, update, and delete pages
- `search` — full-text search across your workspace
- `share` — share pages and API docs
- `tag` — list and manage tags
- `template` — reusable page templates
- `ticket` — read and manage tickets (status, priority, assignee)
- `workspace` — workspace info and roles
- `version` — print the installed version

Each service has its own subcommands; use `mininote <service> --help` to see
them, and `mininote <service> <method> --help` for the flags of a specific
command.
