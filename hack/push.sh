#!/bin/bash

set -e

DEFAULT_REPO="autonomy"

REPO="${1}"
TAG="${2}"


images=( trustd proxyd blockd osd talos )
for i in ${images[@]}; do
    if [ "${REPO}" != "${DEFAULT_REPO}" ]; then
        docker tag ${DEFAULT_REPO}/${i}:${TAG} ${REPO}/${i}:${TAG}
    fi
    docker push ${REPO}/${i}:${TAG}
done

