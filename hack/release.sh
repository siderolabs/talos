#!/usr/bin/env bash

set -e

RELEASE_TOOL_IMAGE="ghcr.io/talos-systems/release-tool:latest"

function release-tool {
  docker pull "${RELEASE_TOOL_IMAGE}" >/dev/null
  docker run --rm -w /src -v "${PWD}":/src:ro "${RELEASE_TOOL_IMAGE}" -l -d -n ${2} -t "${1}" ./hack/release.toml
}

function changelog {
  if [ "$#" -eq 1 ]; then
    (release-tool ${1}; echo; cat CHANGELOG.md) > CHANGELOG.md- && mv CHANGELOG.md- CHANGELOG.md
  else
    echo 1>&2 "Usage: $0 changelog [tag]"
    exit 1
  fi
}

function release-notes {
  release-tool "${2}" --gfm > "${1}"

  echo -e '\n## Images\n\n```' >> ${1}
  ${ARTIFACTS}/talosctl-linux-amd64 images >> ${1}
  echo -e '```\n' >> ${1}

  size=$(stat -c%s "${1}")

  if (( size > 50000 )); then
    echo "Release notes size exceeds GitHub limit of 50000 bytes"
    exit 1
  fi
}

function cherry-pick {
  if [ $# -ne 2 ]; then
    echo 1>&2 "Usage: $0 cherry-pick <commit> <branch>"
    exit 1
  fi

  git checkout $2
  git fetch
  git rebase upstream/$2
  git cherry-pick -x $1
}

function commit {
  if [ $# -ne 1 ]; then
    echo 1>&2 "Usage: $0 commit <tag>"
    exit 1
  fi

  git commit -s -m "release($1): prepare release" -m "This is the official $1 release."
}

if declare -f "$1" > /dev/null
then
  cmd="$1"
  shift
  $cmd "$@"
else
  cat <<EOF
Usage:
  commit:        Create the official release commit message.
  cherry-pick:   Cherry-pick a commit into a release branch.
  changelog:     Update the specified CHANGELOG.
  release-notes: Create release notes for GitHub release.
EOF

  exit 1
fi
