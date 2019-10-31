#!/bin/bash

if [ $# -ne 1 ]; then
  echo 1>&2 "Usage: $0 <tag>"
  exit 1
fi

git commit -s -m "chore: prepare release $1" -m "This is the official $1 release."
