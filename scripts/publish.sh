#!/usr/bin/env bash
#
# publish.sh — assemble the flat "mirror" copy of the mininote CLI for the
# public repo github.com/dakolli/mininote-cli.
#
# The source of truth stays NESTED inside workspace_mininote (the Go module
# lives at mininote-gen/cli). This script whitelist-copies it into a FLAT
# staging dir whose root mirrors the public repo layout: go.mod, go.sum, gen/,
# cmd/, client/, intro.json, api-key-forbidden.txt, README.md, AGENTS.md,
# LICENSE all at the top level, plus the release assets staged in
# mininote-gen/publish/ (.github/, .goreleaser.yaml, install.sh, .gitignore).
#
# The script only SYNCS the staging dir — it never pushes. Committing and
# pushing are deliberately left to the user/planner (the empty GitHub repo is
# created first, then the staged tree is committed and pushed by hand).
#
# Usage:
#   publish.sh                      # stage into <repo>/../publish-out
#   PUBLISH_DIR=/tmp/mirror publish.sh
#
# Env:
#   PUBLISH_DIR   staging dir for the assembled mirror
#                 (default: <mininote-gen>/../publish-out)
#
set -euo pipefail

# --- resolve this script's real location (follow symlinks, runnable from anywhere) ---
SCRIPT_SRC="${BASH_SOURCE[0]}"
while [ -L "$SCRIPT_SRC" ]; do
  DIR="$(cd -P "$(dirname "$SCRIPT_SRC")" >/dev/null 2>&1 && pwd)"
  SCRIPT_SRC="$(readlink "$SCRIPT_SRC")"
  case "$SCRIPT_SRC" in
    /*) ;;
    *) SCRIPT_SRC="$DIR/$SCRIPT_SRC" ;;
  esac
done
SCRIPT_DIR="$(cd -P "$(dirname "$SCRIPT_SRC")" >/dev/null 2>&1 && pwd)"
ROOT_DIR="$(cd -P "$SCRIPT_DIR/.." >/dev/null 2>&1 && pwd)" # mininote-gen/
CLI_DIR="$ROOT_DIR/cli"
PUBLISH_SRC="$ROOT_DIR/publish"
STAGE="${PUBLISH_DIR:-$ROOT_DIR/../publish-out}"

die() {
  echo "error: $*" >&2
  exit 1
}

# --- sanity: the sources we need exist ---
[ -d "$CLI_DIR" ] || die "cli/ not found under $ROOT_DIR"
[ -d "$PUBLISH_SRC" ] || die "publish/ not found under $ROOT_DIR (expected .goreleaser.yaml, install.sh, .github/, .gitignore)"

# --- safety: never stage into the source repo root, and never stage into a
# directory that CONTAINS the source tree (a bad PUBLISH_DIR could point at the
# workspace root, $HOME, or / — rm -rf on it would be catastrophic). ---
[ "$STAGE" = "$ROOT_DIR" ] && die "refusing to stage into the source repo root ($ROOT_DIR)"
case "$ROOT_DIR" in
  "$STAGE"/*) die "refusing to stage into '$STAGE': it contains the source repo" ;;
esac
[ -n "$STAGE" ] && [ "$STAGE" != "/" ] || die "refusing to stage into '$STAGE'"

# --- assemble: clean rebuild keeps the staging dir idempotent ---
rm -rf "$STAGE"
mkdir -p "$STAGE"

echo "==> Staging mirror copy into: $STAGE"

# 1. The Go module, from cli/ to the mirror ROOT (module at root).
#    Whitelist: go.mod, go.sum, gen/, cmd/, client/. Everything else in cli/
#    (e.g. a locally built ./mininote binary) is intentionally NOT copied.
module_items=(go.mod go.sum gen cmd client)
for it in "${module_items[@]}"; do
  [ -e "$CLI_DIR/$it" ] || die "whitelisted source missing: cli/$it"
  cp -a "$CLI_DIR/$it" "$STAGE/$it"
done

# gen/ ships SOURCE only — never generated .gen.go output. The generated code
# that IS published lives in client/ and cmd/ (copied wholesale above).
find "$STAGE/gen" -name '*.gen.go' -type f -delete

# 2. Repo-root data/docs.
for f in intro.json api-key-forbidden.txt README.md AGENTS.md LICENSE; do
  [ -f "$ROOT_DIR/$f" ] || die "whitelisted source missing: $f"
  cp -a "$ROOT_DIR/$f" "$STAGE/$f"
done

# 3. Release assets staged in publish/ (workflows incl. ci.yml, .goreleaser.yaml,
#    install.sh, mirror .gitignore) — merged into the mirror root.
cp -a "$PUBLISH_SRC"/. "$STAGE"/

# --- security sweep: the staging tree must never contain key material ---
if find "$STAGE" \( -name '.env' -o -name '*.pem' -o -name '*.key' -o -name 'id_rsa' -o -name 'id_ed25519' \) -print | grep -q .; then
  die "refusing: staged tree contains key material (.env / *.pem / *.key) — aborting"
fi

# --- summary ---
echo "Synced from cli/ (module now at mirror root):"
printf '  %s\n' "${module_items[@]}"
echo "  (gen/*.gen.go stripped — generated client/ and cmd/ code IS published)"
echo "Synced from $ROOT_DIR:"
printf '  %s\n' intro.json api-key-forbidden.txt README.md AGENTS.md LICENSE
echo "Synced from publish/ (release assets):"
(cd "$PUBLISH_SRC" && find . -type f | sed 's|^\./|  |' | sort)
echo
echo "No .env or key material staged."
echo "Staged $(find "$STAGE" -type f | wc -l | tr -d ' ') files."
echo
echo "Next steps (NOT done by this script — commit/push is manual):"
echo "  cd $STAGE"
echo "  git init -b main"
echo "  git remote add origin git@github.com:dakolli/mininote-cli.git"
echo "  git add -A && git commit -m 'mirror: mininote CLI source + release pipeline'"
echo "  git push -u origin main"
echo
echo "Done. Staging dir: $STAGE"
