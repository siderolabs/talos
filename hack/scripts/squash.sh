#!/bin/bash

set -eou pipefail

docker load < ./images/${2}.tar
# NB: It is up to the caller to ensure that the image name "${1}" matches the image name of what is loaded "${2}".
ID=$(docker create ${1} true)
docker export $ID | docker import - ${1} && docker save ${1} -o ./images/${2}.tar

