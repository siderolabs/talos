#!/bin/bash

docker run --rm -it -v "${PWD}/../../build/osctl-linux-amd64:/bin/osctl:ro" -v "${PWD}/talosconfig:/root/.talos/config" --network dev_talosbr alpine osctl $@
