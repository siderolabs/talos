#!/usr/bin/env bash

for platform in $(tr "," "\n" <<< "${PLATFORM}"); do
    echo ${platform}
    directory="${platform//\//_}"

    if [[ -d "${ARTIFACTS}/${directory}" ]]; then
        mv "${ARTIFACTS}/${directory}/"* ${ARTIFACTS}

        rmdir "${ARTIFACTS}/${directory}/"
    fi
done
