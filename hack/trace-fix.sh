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
# Walks origin/main AND every origin/release-* branch, since fixes are often
# backported as a separate commit with the same subject. Equivalence on a
# backport branch is determined by direct ancestry first, then by commit
# subject (matches conventional cherry-pick workflow).
#
# Usage:
#   trace-fix.sh \
#       --toolchain /path/to/toolchain \
#       --tools     /path/to/tools \
#       --pkgs      /path/to/pkgs \
#       --talos     /path/to/talos \
#       --from <toolchain|tools|pkgs> \
#       --commit <sha> \
#       [--branch <name>]
#
# --branch limits the trace to a single branch (e.g. "main" or "release-1.13").
# Default: walk main + every origin/release-* branch where the fix is present.
# All supplied repos are fetched from origin before queries. Working trees are
# not modified.

set -euo pipefail

usage() {
    { sed -nE 's/^# ?//p' "$0" | head -33; } >&2 || true
    exit 1
}

TOOLCHAIN_PATH=""
TOOLS_PATH=""
PKGS_PATH=""
TALOS_PATH=""
FROM=""
COMMIT=""
BRANCH_FILTER=""

while [ $# -gt 0 ]; do
    case "$1" in
        --toolchain) TOOLCHAIN_PATH="$2"; shift 2;;
        --tools)     TOOLS_PATH="$2"; shift 2;;
        --pkgs)      PKGS_PATH="$2"; shift 2;;
        --talos)     TALOS_PATH="$2"; shift 2;;
        --from)      FROM="$2"; shift 2;;
        --commit)    COMMIT="$2"; shift 2;;
        --branch)    BRANCH_FILTER="$2"; shift 2;;
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

# Fetch origin's `main` plus every `release-*` branch (and all tags) for every
# repo. Queries below run against refs/remotes/origin/{main,release-*} so local
# branch state is irrelevant. Branches outside that set are intentionally not
# fetched — `--branch <name>` is expected to target one of these refs.
sync_repo() {
    name="$1"
    path=$(repo_path "$name")
    echo ">> fetch $name ($path)" >&2
    err=$(git -C "$path" fetch --tags --quiet origin '+refs/heads/main:refs/remotes/origin/main' '+refs/heads/release-*:refs/remotes/origin/release-*' 2>&1) || {
        echo "   WARN: fetch failed for $name (using last pulled refs; results may be stale):" >&2
        printf '   %s\n' "$err" | sed 's/^/     /' >&2
    }
}

for r in $CHAIN; do
    sync_repo "$r"
done

# List branches to check in source repo: main + every origin/release-*
# (sorted), respecting --branch filter.
list_source_branches() {
    src="$1"
    {
        git -C "$src" show-ref --verify --quiet refs/remotes/origin/main && echo "main"
        git -C "$src" for-each-ref --format='%(refname:short)' 'refs/remotes/origin/release-*' \
            | sed 's|^origin/||' \
            | sort -V
    }
}

branch_exists() {
    repo="$1"; branch="$2"
    git -C "$repo" show-ref --verify --quiet "refs/remotes/origin/$branch"
}

# Find equivalent of <target> on <branch> in <repo>.
# 1. Direct ancestry: target itself.
# 2. Exact-subject match: first commit on branch whose %s equals target's %s.
#    (Walks `git log --format='%H<TAB>%s'` rather than using `--grep`, which
#    matches the whole message and would surface unrelated commits with a
#    similar substring or body line. Tab is the separator because BSD awk
#    doesn't accept NUL as FS; commit subjects never contain tabs in
#    practice.)
# Echoes full sha on success.
equivalent_on_branch() {
    repo="$1"; branch="$2"; target="$3"
    if git -C "$repo" merge-base --is-ancestor "$target" "origin/$branch" 2>/dev/null; then
        echo "$target"
        return 0
    fi
    subj=$(git -C "$repo" log -1 --format='%s' "$target")
    eq=$(
        git -C "$repo" log --format='%H%x09%s' "origin/$branch" 2>/dev/null \
        | awk -v subj="$subj" -F '\t' '$2 == subj { print $1; exit }'
    )
    [ -n "$eq" ] && { echo "$eq"; return 0; }
    return 1
}

# Resolve a pin string to a full SHA in the upstream repo. Empty on failure.
resolve_ref() {
    repo="$1"; ref="$2"
    suffix=$(printf '%s\n' "$ref" | sed -nE 's/.*-g([0-9a-fA-F]+)$/\1/p')
    if [ -n "$suffix" ]; then
        git -C "$repo" rev-parse "$suffix" 2>/dev/null || true
    else
        git -C "$repo" rev-parse "$ref" 2>/dev/null || true
    fi
}

# Walk commits on origin/<branch> that touched <file>, oldest first.
pin_history() {
    repo="$1"; branch="$2"; file="$3"; sed_extract="$4"
    shas=$(git -C "$repo" log --reverse --format='%H' "origin/$branch" -- "$file")
    for sha in $shas; do
        val=$(git -C "$repo" show "$sha:$file" 2>/dev/null \
            | { sed -nE "$sed_extract" || true; } \
            | { head -1 || true; })
        [ -n "$val" ] && echo "$sha $val"
    done
}

# Find first commit on origin/<branch> in <dep_repo> whose pin resolves to a
# ref that has <target> as an ancestor in <up_repo>. Echoes "<sha> <pin>".
find_first_containing() {
    dep_repo="$1"; branch="$2"; dep_file="$3"; sed_extract="$4"; up_repo="$5"; target="$6"
    tmp=$(mktemp)
    pin_history "$dep_repo" "$branch" "$dep_file" "$sed_extract" > "$tmp"
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

# Trace one chain on a single branch. Args: branch, source-sha-on-branch.
trace_chain() {
    branch="$1"; src_sha="$2"

    echo
    echo "==========================================================="
    echo " branch: $branch"
    echo "==========================================================="

    print_commit_block "Source ($SRC_REPO @ $branch)" "$SRC_REPO" "$src_sha"

    CARRY_REPO="$SRC_REPO"
    CARRY_SHA="$src_sha"

    set -- $CHAIN
    shift
    for DEP in "$@"; do
        DEP_PATH=$(repo_path "$DEP")

        if ! branch_exists "$DEP_PATH" "$branch"; then
            echo
            echo "  $DEP: no origin/$branch branch — chain stops here."
            return 0
        fi

        case "$DEP" in
            tools) DEP_FILE="Pkgfile";  SED="$SED_TOOLS_PIN";;
            pkgs)  DEP_FILE="Pkgfile";  SED="$SED_PKGS_PIN";;
            talos) DEP_FILE="Makefile"; SED="$SED_TALOS_PIN";;
        esac

        CARRY_PATH=$(repo_path "$CARRY_REPO")

        if ! result=$(find_first_containing "$DEP_PATH" "$branch" "$DEP_FILE" "$SED" "$CARRY_PATH" "$CARRY_SHA"); then
            short=$(git -C "$CARRY_PATH" rev-parse --short "$CARRY_SHA")
            echo
            echo "  $DEP @ $branch: no pin yet contains $CARRY_REPO commit $short."
            return 0
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

        print_commit_block "$DEP @ $branch (first bump containing fix)" "$DEP" "$DEP_SHA" "$PIN_VAL" "$EXTRA"

        CARRY_REPO="$DEP"
        CARRY_SHA="$DEP_SHA"
    done
}

SRC_REPO="$FROM"
SRC_PATH=$(repo_path "$SRC_REPO")
if ! SRC_SHA=$(git -C "$SRC_PATH" rev-parse "$COMMIT" 2>/dev/null); then
    echo "commit $COMMIT not found in $SRC_REPO at $SRC_PATH" >&2
    exit 1
fi

# Determine which branches to check.
if [ -n "$BRANCH_FILTER" ]; then
    BRANCHES_TO_CHECK="$BRANCH_FILTER"
else
    BRANCHES_TO_CHECK=$(list_source_branches "$SRC_PATH")
fi

# For each branch, compute equivalent commit. Print summary, then traces.
PRESENT_BRANCHES=""
declare -a TRACES_BRANCH TRACES_SHA
for b in $BRANCHES_TO_CHECK; do
    if ! branch_exists "$SRC_PATH" "$b"; then
        echo "WARN: origin/$b not in $SRC_REPO — skipping." >&2
        continue
    fi
    if eq=$(equivalent_on_branch "$SRC_PATH" "$b" "$SRC_SHA"); then
        PRESENT_BRANCHES="$PRESENT_BRANCHES $b"
        TRACES_BRANCH+=("$b")
        TRACES_SHA+=("$eq")
    fi
done

if [ ${#TRACES_BRANCH[@]} -eq 0 ]; then
    echo "fix not found on any checked branch in $SRC_REPO." >&2
    exit 2
fi

echo
echo "Fix present on branches:$PRESENT_BRANCHES"

i=0
while [ $i -lt ${#TRACES_BRANCH[@]} ]; do
    trace_chain "${TRACES_BRANCH[$i]}" "${TRACES_SHA[$i]}"
    i=$((i + 1))
done

echo
