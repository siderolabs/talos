#!/bin/bash

set -e

CGO_ENABLED=1
GOPACKAGES=$(go list ./...)

lint_packages() {
  echo "Linting packages"
  golangci-lint run --config ${1}
}

perform_unit_tests() {
  echo "Performing unit tests"
  go test -v -short ./...
}

perform_integration_tests() {
  echo "Performing integration tests"
  go test -v ./...
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
  --lint)
  lint_packages ${2}
  ;;
  --unit)
  perform_unit_tests
  ;;
  --integration)
  perform_integration_tests
  ;;
  --coverage)
  perform_coverage_tests
  ;;
  *)
  exit 1
  ;;
esac

exit 0
