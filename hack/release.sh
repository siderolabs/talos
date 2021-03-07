#!/bin/bash

set -e

function changelog {
  if [ "$#" -eq 1 ]; then
    git-chglog --output CHANGELOG.md -c ./hack/chglog/config.yml --tag-filter-pattern "^${1}" "${1}.0-alpha.1.."
  elif [ "$#" -eq 0 ]; then
    git-chglog --output CHANGELOG.md -c ./hack/chglog/config.yml
  else
    echo 1>&2 "Usage: $0 changelog [tag]"
    exit 1
  fi
}

function release-notes {
  git-chglog --output ${1} -c ./hack/chglog/config.yml "${2}"

  echo -e '## Images\n\n```' >> ${1}
  ${ARTIFACTS}/talosctl-linux-amd64 images >> ${1}
  echo -e '```\n' >> ${1}
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
  commit:       Create the official release commit message.
  cherry-pick:   Cherry-pick a commit into a release branch.
  changelog:    Update the specified CHANGELOG.
EOF

  exit 1
fi
