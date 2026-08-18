# mininote

A CLI for [mininote.ink](https://mininote.ink/).

> [mininote.ink](https://mininote.ink/) is a fast, no-nonsense notes-and-wiki app built on one idea: you should own what you write. Plain markdown, instant sync across every device, and a door that's always open — export everything, any time, no questions asked.
> Created by [@toyz](https://github.com/toyz).

## Overview

`mininote` maps 1:1 to the [MiniNote RPC API](https://mininote.ink/docs/rpc-api). Every backend endpoint is exposed as a `mininote <service> <method>` command, mapping flags and positional arguments cleanly to parameters.

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
mininote config add-token mnk_... --name primary
```

The key is saved in the local key vault at `~/.config/mininote/mininote.db` with user-only permissions (`0600`). You can store multiple named keys (RPC or MCP) and switch between them:

```sh
mininote config add-token mnk_... --name work --workspace WORK
mininote config list-tokens             # list all stored keys
mininote config use work                # switch active key
mininote config current                 # view current active key
```

For temporary sessions or CI, set `export MININOTE_RPC_KEY="mnk_..."` or pass `--token` / `-t` directly. You can also select a specific key per-command using `--key <name>` / `-k <name>`.

## Quick Start

Primary required arguments support positional syntax or short flags (`-q`, `-i`, `-p`, `-b`, etc.):

```sh
mininote page tree                                # list pages
mininote page get p_abc123                        # fetch a page by ID (or -i)
mininote search query "notes"                     # full-text search (or -q)
mininote page upsert docs/readme "# Hello"        # create or update (-p path, -b body)
mininote workspace forKey                         # list workspaces
```

Output is indented JSON by default. Add `--compact` for single-line JSON.

## Authentication & Configuration

| Auth Method | How |
|---|---|
| Flag Override | `--token` / `-t` flag (raw token) or `--key` / `-k` flag (named key from vault) |
| Environment | `MININOTE_RPC_KEY` or `MININOTE_TOKEN` |
| Key Vault | `~/.config/mininote/mininote.db` (managed via `mininote config`) |
| Output | `--compact` for single-line JSON |

Precedence: `--token` flag > Environment variables > `--key` flag > Active vault key > Single stored key fallback.

## Commands by Service

Run `mininote <service> --help` or `mininote <service> <method> --help` for flag details. Full endpoint schemas are documented in the [RPC API docs](https://mininote.ink/docs/rpc-api).

### `page`
Read, create, update, upsert, and delete notes/pages.
- `tree` — list page hierarchy (`mininote page tree`)
- `get` — fetch page details (`mininote page get <id>` or `-i <id>`)
- `upsert` — create or update a page (`mininote page upsert <path> <body>` or `-p <path> -b <body>`)
- `delete` — delete a page (`mininote page delete <id>`)
- `listPrefix` — list pages under a directory path (`mininote page listPrefix -p <path>`)
- `changes`, `pathOf`, `refs`, `resolvePath`, `export`, `exportAll`, `import`, `clone`

### `search`
Full-text search across your workspace.
- `query` — search note contents (`mininote search query <query>` or `-q <query> [-k kinds] [-l limit]`)

### `ticket`
Track and manage task tickets attached to pages.
- `get` — fetch ticket (`mininote ticket get <node_id>` or `-n <node_id>`)
- `set` — update ticket metadata (`mininote ticket set -n <node_id> -s <status> -p <priority> -o <owner>`)
- `patch` — partial update of ticket fields
- `changes`, `history`, `delete`, `restore`

### `tag`
Manage tags and note taxonomy.
- `list` — list all workspace tags (`mininote tag list`)
- `create` / `update` / `delete` — manage tag taxonomy
- `setNodeTags` — assign tags to a note (`mininote tag setNodeTags -n <node_id> -t <tag_ids>`)

### `workspace`
Workspace management.
- `forKey` — list accessible workspaces and active workspace info (`mininote workspace forKey`)

### `workspaceTemplate`
Reusable workspace-scoped page templates (saved blueprints).
- `list` — list workspace templates (`mininote workspaceTemplate list`)
- `save` — save the current page tree as a reusable blueprint (`mininote workspaceTemplate save <name>`, `--structure_only` to copy tree/layout without bodies)
- `delete` — delete a workspace template (`mininote workspaceTemplate delete <id>`)

### `comment`
Read and write discussion comments on notes.
- `list` — list comments for a page (`mininote comment list -p <page_id>`)
- `add` — post a new comment (`mininote comment add -p <page_id> -b <body>`)
- `edit` / `delete` — manage comments

### `annotation`
Read and manage inline annotations on notes.
- `list` / `add` / `resolve` / `delete` — inline note annotations

### `history`
Track and restore page revision history.
- `list` — list revisions (`mininote history list -n <node_id>`)
- `revision` — fetch revision diff (`mininote history revision -i <revision_id>`)
- `restore` — revert a page to a previous revision (`mininote history restore -i <revision_id>`)

### `template`
Reusable page templates.
- `instantiate` — create a new page from a template (`mininote template instantiate -t <template_id> -p <path>`)

### `share`
Manage public page shares and API docs.
- `mine` — list active public shares
- `create` / `status` / `revoke` / `setPassword` — configure public links and access control

### `event` & `calFeed`
Calendar integration and iCal feeds.
- `event` — schedule and manage calendar events attached to notes
- `calFeed` — generate iCal feed URLs for calendar syncing

### `export`
Batch export workspace notes and documents.
- `page` — export a page to Markdown/HTML/JSON
- `prepare` — generate export archives
