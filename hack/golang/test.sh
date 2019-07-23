#!/bin/sh

set -e

perform_tests() {
  echo "Performing tests on $1"
  go test -v -covermode=atomic -coverprofile=coverage.txt "$1"
}

perform_short_tests() {
  echo "Performing short tests on $1"
  go test -v -short "$1"
}

case $1 in
  --short)
  shift
  perform_short_tests "${1:-./...}"
  ;;
  *)
  perform_tests "${1:-./...}"
  ;;
esac

exit 0
