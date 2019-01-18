#!/bin/bash

set -e

CGO_ENABLED=1
GOPACKAGES=$(go list ./...)

perform_tests() {
  echo "Performing tests"
  go test -v ./...
}

perform_short_tests() {
  echo "Performing short tests"
  go test -v -short ./...
}

perform_coverage_tests() {
  echo "Performing coverage tests"
  local coverage_report="coverage.txt"
  local profile="profile.out"
  if [[ -f ${coverage_report} ]]; then
    rm ${coverage_report}
  fi
  touch ${coverage_report}
  for package in ${GOPACKAGES[@]}; do
    go test -v -short -race -coverprofile=${profile} -covermode=atomic $package
    if [ -f ${profile} ]; then
      cat ${profile} >> ${coverage_report}
      rm ${profile}
    fi
  done
}

case $1 in
  --short)
  perform_short_tests
  ;;
  --coverage)
  perform_coverage_tests
  ;;
  *)
  perform_tests
  ;;
esac

exit 0
