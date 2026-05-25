#!/usr/bin/env bash
#
# trace-fix.sh — trace a fix commit from an upstream sidero repo (toolchain /
# tools / pkgs) down through the dependency chain and into talos.
#
# Chain:
#   toolchain --(TOOLCHAIN_IMAGE in tools/Pkgfile)--> tools
#   tools     --(TOOLS_REV in pkgs/Pkgfile)--------> pkgs
#   pkgs      --(PKGS in talos/Makefile)-----------> talos
#
# For each hop, finds the first dependent-repo commit whose pin resolves to a
# ref that has the fix commit as an ancestor.
#
# Usage:
#   trace-fix.sh \
#       --toolchain /path/to/toolchain \
#       --tools     /path/to/tools \
#       --pkgs      /path/to/pkgs \
#       --talos     /path/to/talos \
#       --from <toolchain|tools|pkgs> \
#       --commit <sha>
#
# Only the repos needed for the chain from --from down to talos must be
# supplied. All supplied repos are fetched from origin/main before queries.
# Working trees are not modified.

set -euo pipefail

usage() {
    { sed -nE 's/^# ?//p' "$0" | head -30; } >&2 || true
    exit 1
}

TOOLCHAIN_PATH=""
TOOLS_PATH=""
PKGS_PATH=""
TALOS_PATH=""
FROM=""
COMMIT=""

while [ $# -gt 0 ]; do
    case "$1" in
        --toolchain) TOOLCHAIN_PATH="$2"; shift 2;;
        --tools)     TOOLS_PATH="$2"; shift 2;;
        --pkgs)      PKGS_PATH="$2"; shift 2;;
        --talos)     TALOS_PATH="$2"; shift 2;;
        --from)      FROM="$2"; shift 2;;
        --commit)    COMMIT="$2"; shift 2;;
        -h|--help)   usage;;
        *) echo "unknown arg: $1" >&2; usage;;
    esac
done

[ -z "$FROM" ] && usage
[ -z "$COMMIT" ] && usage

repo_path() {
    case "$1" in
        toolchain) echo "$TOOLCHAIN_PATH";;
        tools)     echo "$TOOLS_PATH";;
        pkgs)      echo "$PKGS_PATH";;
        talos)     echo "$TALOS_PATH";;
        *) echo "unknown repo: $1" >&2; exit 1;;
    esac
}

repo_url() {
    case "$1" in
        toolchain) echo "https://github.com/siderolabs/toolchain";;
        tools)     echo "https://github.com/siderolabs/tools";;
        pkgs)      echo "https://github.com/siderolabs/pkgs";;
        talos)     echo "https://github.com/siderolabs/talos";;
    esac
}

case "$FROM" in
    toolchain) CHAIN="toolchain tools pkgs talos";;
    tools)     CHAIN="tools pkgs talos";;
    pkgs)      CHAIN="pkgs talos";;
    *) echo "--from must be one of: toolchain | tools | pkgs" >&2; exit 1;;
esac

for r in $CHAIN; do
    p=$(repo_path "$r")
    [ -z "$p" ] && { echo "missing --$r PATH" >&2; exit 1; }
    git -C "$p" rev-parse --is-inside-work-tree >/dev/null 2>&1 \
        || { echo "$p is not a git repo" >&2; exit 1; }
done

# Fetch origin/main + tags for every repo. Queries run against origin/main so
# local branch state is irrelevant and never modified.
sync_repo() {
    name="$1"
    path=$(repo_path "$name")
    echo ">> fetch $name ($path)" >&2
    err=$(git -C "$path" fetch --tags --quiet origin main 2>&1) || {
        echo "   WARN: fetch failed for $name (using last pulled origin/main; results may be stale):" >&2
        printf '   %s\n' "$err" | sed 's/^/     /' >&2
    }
    local_branch=$(git -C "$path" rev-parse --abbrev-ref HEAD 2>/dev/null || echo "DETACHED")
    if [ "$local_branch" != "main" ]; then
        echo "   note: $name working tree on '$local_branch' (queries use origin/main)" >&2
    fi
}

for r in $CHAIN; do
    sync_repo "$r"
done

# Resolve a pin string (e.g. "v1.14.0-alpha.0-2-gd6c7ac5" or "v1.14.0-alpha.0")
# to a full SHA in the upstream repo. Empty on failure.
resolve_ref() {
    repo="$1"; ref="$2"
    suffix=$(printf '%s\n' "$ref" | sed -nE 's/.*-g([0-9a-fA-F]+)$/\1/p')
    if [ -n "$suffix" ]; then
        git -C "$repo" rev-parse "$suffix" 2>/dev/null || true
    else
        git -C "$repo" rev-parse "$ref" 2>/dev/null || true
    fi
}

# Walk commits on origin/main that touched <file>, oldest first, emitting
# "<sha> <pin-value>" lines. <sed_extract> is a sed -nE script that prints the
# pin value when applied to the file content.
pin_history() {
    repo="$1"; file="$2"; sed_extract="$3"
    shas=$(git -C "$repo" log --reverse --format='%H' origin/main -- "$file")
    for sha in $shas; do
        val=$(git -C "$repo" show "$sha:$file" 2>/dev/null | sed -nE "$sed_extract" | head -1 || true)
        [ -n "$val" ] && echo "$sha $val"
    done
}

# Given dependent repo + pin location + upstream repo + target commit, find
# the first dependent-repo commit whose pin resolves to a ref that has
# <target> as an ancestor. Echoes "<dep-sha> <pin-value>" on success.
find_first_containing() {
    dep_repo="$1"; dep_file="$2"; sed_extract="$3"; up_repo="$4"; target="$5"
    tmp=$(mktemp)
    pin_history "$dep_repo" "$dep_file" "$sed_extract" > "$tmp"
    prev_val=""
    rc=1
    while read -r sha val; do
        [ "$val" = "$prev_val" ] && continue
        prev_val="$val"
        resolved=$(resolve_ref "$up_repo" "$val")
        [ -z "$resolved" ] && continue
        if git -C "$up_repo" merge-base --is-ancestor "$target" "$resolved" 2>/dev/null; then
            echo "$sha $val"
            rc=0
            break
        fi
    done < "$tmp"
    rm -f "$tmp"
    return $rc
}

# sed extractions for each pin location.
SED_TOOLS_PIN='s|^[[:space:]]*TOOLCHAIN_IMAGE:[[:space:]]*ghcr\.io/siderolabs/toolchain:([^[:space:]]+).*$|\1|p'
SED_PKGS_PIN='s|^[[:space:]]*TOOLS_REV:[[:space:]]*([^[:space:]]+).*$|\1|p'
SED_TALOS_PIN='s|^PKGS[[:space:]]*\?=[[:space:]]*([^[:space:]]+).*$|\1|p'

print_commit_block() {
    label="$1"; repo="$2"; sha="$3"; pin="${4:-}"; extra="${5:-}"
    path=$(repo_path "$repo")
    short=$(git -C "$path" rev-parse --short "$sha")
    subject=$(git -C "$path" log -1 --format='%s' "$sha")
    echo
    echo "$label"
    echo "  commit:  $(repo_url "$repo")/commit/$sha"
    echo "           $short $subject"
    [ -n "$pin" ]   && echo "  pin set: $pin"
    [ -n "$extra" ] && printf '%s\n' "$extra"
    return 0
}

# Resolve full SHA of source commit in source repo.
SRC_REPO="$FROM"
SRC_PATH=$(repo_path "$SRC_REPO")
if ! SRC_SHA=$(git -C "$SRC_PATH" rev-parse "$COMMIT" 2>/dev/null); then
    echo "commit $COMMIT not found in $SRC_REPO at $SRC_PATH" >&2
    exit 1
fi

print_commit_block "Source ($SRC_REPO)" "$SRC_REPO" "$SRC_SHA"

# Walk the chain. CARRY_REPO/CARRY_SHA is the upstream commit we're looking
# for in the next-down repo's pin.
CARRY_REPO="$SRC_REPO"
CARRY_SHA="$SRC_SHA"

set -- $CHAIN
shift  # drop source repo; iterate dependents
for DEP in "$@"; do
    DEP_PATH=$(repo_path "$DEP")

    case "$DEP" in
        tools) DEP_FILE="Pkgfile";  SED="$SED_TOOLS_PIN";;
        pkgs)  DEP_FILE="Pkgfile";  SED="$SED_PKGS_PIN";;
        talos) DEP_FILE="Makefile"; SED="$SED_TALOS_PIN";;
    esac

    CARRY_PATH=$(repo_path "$CARRY_REPO")

    if ! result=$(find_first_containing "$DEP_PATH" "$DEP_FILE" "$SED" "$CARRY_PATH" "$CARRY_SHA"); then
        short=$(git -C "$CARRY_PATH" rev-parse --short "$CARRY_SHA")
        echo >&2
        echo "$DEP: no pin on origin/main yet contains $CARRY_REPO commit $short." >&2
        exit 2
    fi

    DEP_SHA=$(echo "$result" | awk '{print $1}')
    PIN_VAL=$(echo "$result" | awk '{print $2}')

    EXTRA=""
    if [ "$DEP" = "talos" ]; then
        DESC=$(git -C "$DEP_PATH" describe --tags "$DEP_SHA" 2>/dev/null || true)
        CONTAIN=$(git -C "$DEP_PATH" tag --contains "$DEP_SHA" 2>/dev/null \
            | { paste -sd ',' - || true; } \
            | sed 's/,/, /g')
        REL=$(git -C "$DEP_PATH" describe --contains "$DEP_SHA" 2>/dev/null || true)
        N_BACK=$(printf '%s\n' "$DESC" | sed -nE 's/.*-([0-9]+)-g[0-9a-fA-F]+$/\1/p')
        N_FWD=$(printf '%s\n' "$REL"  | sed -nE 's/.*~([0-9]+).*/\1/p')
        EXTRA="  describe:        $DESC (${N_BACK:-?} commits AFTER the nearest ancestor tag — that tag does NOT ship this commit)
  containing tags: ${CONTAIN:-<none>}
  position:        $REL (${N_FWD:-?} first-parent commits BEFORE the containing tag — that release SHIPS this commit)"
    fi

    print_commit_block "$DEP (first bump containing fix)" "$DEP" "$DEP_SHA" "$PIN_VAL" "$EXTRA"

    CARRY_REPO="$DEP"
    CARRY_SHA="$DEP_SHA"
done

echo
