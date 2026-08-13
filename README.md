# mininote

A typed command-line client for [mininote.ink](https://mininote.ink/).

> [mininote.ink](https://mininote.ink/) is a fast, no-nonsense notes-and-wiki app built on one idea: you should own what you write. Plain markdown, instant sync across every device, and a door that's always open — export everything, any time, no questions asked.
> Created by [@toyz](https://github.com/toyz).

## Overview

`mininote` maps 1:1 to the [MiniNote RPC API](https://mininote.ink/docs/rpc-api). Every backend endpoint is exposed as a `mininote <service> <method>` command, mapping flags to parameters.

## Install

**Linux / macOS**:

```sh
curl -sfL https://raw.githubusercontent.com/dakolli/mininote-cli/main/install.sh | sh
```

**From source**:

```sh
go install github.com/dakolli/mininote-cli@latest
```

Prebuilt binaries for Linux, macOS, and Windows are available on [GitHub Releases](https://github.com/dakolli/mininote-cli/releases).

## Setup

Grab a workspace API key (`mnk_...`) with REST/RPC API access enabled from your workspace settings and save it:

```sh
mininote config set-token mnk_...
```

That's it — the key is stored in `~/.config/mininote/cli.json` and used automatically.

## Quick Start

```sh
mininote page tree                                # list pages
mininote page get --id p_abc123                   # fetch a page
mininote search query --query notes               # full-text search
mininote page upsert --path docs/readme --body '# Hello'   # create or update
mininote workspace forKey                         # list workspaces
```

Output is indented JSON by default. Add `--compact` for single-line JSON.

## Configuration

| Setting | How |
|---|---|
| Server | `--base-url` (default `https://mininote.ink`), or `MININOTE_BASE_URL` |
| Auth | `--token` flag, `~/.config/mininote/cli.json`, or env `MININOTE_RPC_KEY` / `MININOTE_TOKEN` |
| Output | `--compact` for single-line JSON |

Precedence: Flags > Config file > Environment variables.

## Commands

- `annotation` — inline annotations
- `calFeed` — iCal calendar feeds
- `comment` — page comments
- `event` — calendar events
- `export` — page and document exports
- `history` — revision history
- `page` — read, create, update, delete, and upsert pages
- `search` — full-text search
- `share` — page sharing and API docs
- `tag` — manage tags
- `template` — page templates
- `ticket` — manage tickets (status, priority, assignee)
- `workspace` — workspace info and roles
- `version` — print CLI version

Run `mininote <service> --help` or `mininote <service> <method> --help` for flags. Full endpoint schemas are documented in the [RPC API docs](https://mininote.ink/docs/rpc-api).
