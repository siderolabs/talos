#!/bin/bash

set -e

while getopts c flag
do
    case "${flag}" in
        c) make clean;;
    esac
done

make release-artifacts
make USERNAME=andrewrynhard TAG="${1}" installer talosctl _out/integration-test-provision-linux-amd64
docker push andrewrynhard/installer:"${1}"
sudo -E _out/integration-test-provision-linux-amd64 \
  -talos.name local \
  -talos.state /tmp/local \
  -test.v \
  -talos.crashdump=false \
  -talos.talosctlpath=$PWD/_out/talosctl-linux-amd64 \
  -test.run "TestIntegration/provision.UpgradeSuite.v0.6.0-beta.2-${1}"
