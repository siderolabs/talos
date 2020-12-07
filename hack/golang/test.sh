#!/bin/sh

set -e

# Set up common test environment variables
export PLATFORM=container

perform_tests() {
  echo "Performing tests on $1"
  go test -v -covermode=atomic -coverprofile=coverage.txt -count 1 -p 4 "$1"
}

perform_race_tests() {
  echo "Performing race tests on $1"
  CGO_ENABLED=1 go test -v -race -count 1 -p 4 "$1"
}

perform_short_tests() {
  echo "Performing short tests on $1"
  go test -v -short -count 1 -p 4 "$1"
}

case $1 in
  --race)
  shift
  perform_race_tests "${1:-./...}"
  ;;
  --short)
  shift
  perform_short_tests "${1:-./...}"
  ;;
  *)
  perform_tests "${1:-./...}"
  ;;
esac

exit 0
