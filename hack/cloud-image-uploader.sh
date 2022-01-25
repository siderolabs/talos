#!/usr/bin/env bash

set -e

cd hack/cloud-image-uploader

go run . --artifacts-path="../../${ARTIFACTS}" --tag="${TAG}"
