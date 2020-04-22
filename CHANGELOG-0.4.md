# [v0.4.1](https://github.com/talos-systems/talos/compare/v0.4.0...v0.4.1) (2020-04-22)

### Bug Fixes

- ensure disk is not busy ([7f78d37](https://github.com/talos-systems/talos/commit/7f78d37512021215a84348d6f2734f2976ecb146))
- pass dev path to mkfs ([1cd1256](https://github.com/talos-systems/talos/commit/1cd1256faabc2496d4bad110255da128875622b7))
- prevent formatting the ephemeral partition twice ([6ba4d50](https://github.com/talos-systems/talos/commit/6ba4d50e26a839640590fb2aacc1f60616ad51b6))

# [v0.4.0](https://github.com/talos-systems/talos/compare/v0.4.0-rc.0...v0.4.0) (2020-04-17)

### Bug Fixes

- ensure ordering of interfaces when deciding hostname ([ec695ac](https://github.com/talos-systems/talos/commit/ec695ac7c4b46ed2d37602860f30ee416c948419))
- set ephemeral partition to max size ([df4fd65](https://github.com/talos-systems/talos/commit/df4fd6572298a64bae9adda5b43f0b3fa3f8c3c2))

### Features

- add extra headers to fetch of extraManifests ([c78e937](https://github.com/talos-systems/talos/commit/c78e93779b335afc04b0a8b69adcff6b4e0ff350))
- disable kubelet ro port ([3f49871](https://github.com/talos-systems/talos/commit/3f4987165eccd94a6c206efd82102a8fae8dcf23))

<a name="v0.4.0-rc.0"></a>

## [v0.4.0-rc.0](https://github.com/talos-systems/talos/compare/v0.4.0-beta.1...v0.4.0-rc.0) (2020-04-14)

### Chore

- update pkgs tag to v0.2.0
- address random CI nits

### Feat

- upgrade Go to 1.14.2

### Fix

- resolve race condition in createNodes
- add hpsa drivers

### Refactor

- don't log installer verification

### Test

- serialize `docs` step execution
- update versions used for upgrade tests

<a name="v0.4.0-beta.1"></a>

## [v0.4.0-beta.1](https://github.com/talos-systems/talos/compare/v0.4.0-beta.0...v0.4.0-beta.1) (2020-04-07)

### Chore

- prepare release v0.4.0-beta.1
- update sonobuoy to v0.18.0
- update timeout values for e2e tests

### Feat

- upgrade Linux to v5.5.15

### Fix

- add bnx2 and bnx2x firmware
- wait for `system-containerd` to become healthy before proceeding
- mount TLS certs into bootkube container
- make sure Close() is called on every path

<a name="v0.4.0-beta.0"></a>

## [v0.4.0-beta.0](https://github.com/talos-systems/talos/compare/v0.4.0-alpha.8...v0.4.0-beta.0) (2020-04-03)

### Chore

- prepare release v0.4.0-beta.0

### Docs

- Add example of a VLAN configured device.

### Feat

- add BNX drivers
- introduce ability to specify extra hosts in /etc/hosts
- allow for exposing ports on docker clusters
- move bootkube out as full service
- upgrade kubernetes to 1.18
- make `--wait` default option to `talosctl cluster create`

### Fix

- delete tag on revert with empty label
- move empty label check
- wait for USB storage
- ignore EINVAL on unmounting when mount point isn't mounted
- make upgrades work with UEFI
- don't use ARP table for networkd health check

### Refactor

- move Talos client package to `pkg/`
- include partition label when unmount fails

### Test

- mark long tests as !short

<a name="v0.4.0-alpha.8"></a>

## [v0.4.0-alpha.8](https://github.com/talos-systems/talos/compare/v0.4.0-alpha.7...v0.4.0-alpha.8) (2020-03-24)

### Chore

- prepare release v0.4.0-alpha.8
- update upgrade tests for new version, split into two tracks
- run npm audit fix

### Docs

- add bare-metal install example yaml

### Feat

- update bootkube
- add usb storage support
- initial work for supporting vlans
- build talosctl for ARM v7
- build talosctl for ARM64

### Fix

- update k8s to 1.17.3
- update rtnetlink checks for bit masks

<a name="v0.4.0-alpha.7"></a>

## [v0.4.0-alpha.7](https://github.com/talos-systems/talos/compare/v0.4.0-alpha.6...v0.4.0-alpha.7) (2020-03-20)

### Chore

- prepare release v0.4.0-alpha.7
- fix formatting of imports
- update Firecracker Go SDK to the official release
- cleanup assets dir after bootkube is done
- improve handling of etcd responses in bootkube pre-func
- add service state to postfunc

### Docs

- update the website generator's npm packages

### Feat

- rename osctl to talosctl
- add support for `--with-debug` to osctl cluster create
- split `osctl` commands into Talos API and cluster management
- upgrade Go to version 1.14.1
- update talos base packages
- add debug logs to networkd health check
- respect panic kernel flag
- allow for persistence of config data
- split routerd from apid
- make admin kubeconfig cert lifetime configurable
- add function for mounting a specific system disk partition
- generate kubeconfig on the fly on request

### Fix

- respect dns domain from machine config
- ensure printing of panic message
- add debug option to v1alpha1 config
- skip links without a carrier
- ensure hostname is never empty
- ensure CA cert generation respects the hour flag

### Refactor

- perform upgrade upon reboot

### Test

- add test for empty hostname option
- add 'reset' integration test for Reset() API

### BREAKING CHANGE

This PR fixes a bug where we were only passing `cluster.local` to the
kubelet configuration. It will also pull in a new version of the
bootkube fork to ensure that custom domains got propogated down to the
API Server certs, as well as the CoreDNS configuration for a cluster.

Existing users should be aware that, if they were previously trying to
use this option in machine configs, that an upgrade will may break
their cluster. It will update a kubelet flag with the new domain, but
CoreDNS and API Server certs will not change since bootkube has already
run. One option may be to change these values manually inside the
Kubernetes cluster. However, it may prove easier to rebuild the cluster
if necessary.

Additionally, this PR also exposes a flag to `osctl config generate`
to allow tweaking this domain value as well.

<a name="v0.4.0-alpha.6"></a>

## [v0.4.0-alpha.6](https://github.com/talos-systems/talos/compare/v0.4.0-alpha.5...v0.4.0-alpha.6) (2020-02-27)

### Chore

- prepare release v0.4.0-alpha.6
- update pkgs & tools for Go 1.14
- fix small misprint
- push installer & talos images to the CI registry on every build
- move golangci-lint.yaml to .golangci.yml
- remove KubernetesVersion from provision request

### Feat

- support proxy in docker buildx
- support sending machine info
- add reboot flag to reset API

### Fix

- ensre proxy is used when fetching additional manifests for bootkube
- unmount bind mounts for system (fixes upgrade stuck on disk busy)
- refresh proxy settings from environment in image resolver
- default reboot flag to false
- add reboot flag to reset command
- stop firecracker launcher on signal
- fix reset command
- allow kublet to handle multiple service CIDRs
- validate install disk
- PodCIDR, ServiceCIDR should be comma sets

### Refactor

- use go-procfs

### Test

- enable upgrade tests 0.4.x -> latest
- implement new class of tests: provision tests (upgrades)
- fix `RebootAllNodes` test to reboot all nodes in one call
- implement RebootAllNodes test

<a name="v0.4.0-alpha.5"></a>

## [v0.4.0-alpha.5](https://github.com/talos-systems/talos/compare/v0.4.0-alpha.4...v0.4.0-alpha.5) (2020-02-15)

### Chore

- prepare release v0.4.0-alpha.5
- build app container images skipping export to host
- update pkgs
- support bootloader emulation in firecracker provisioner
- implement loadbalancer for firecracker provisioner

### Feat

- implement registry mirror & config for image pull

### Fix

- don't proxy gRPC unix connections
- do not add empty netconf

<a name="v0.4.0-alpha.4"></a>

## [v0.4.0-alpha.4](https://github.com/talos-systems/talos/compare/v0.4.0-alpha.3...v0.4.0-alpha.4) (2020-02-04)

### Chore

- remove Firecracker bridge interface in osctl cluster destroy
- sign .drone.yml
- only run ok-to-test when PR
- support slash commands in drone
- get correct drone status in github actions
- use upstream version of Firecracker Go SDK
- update golangci-lint-1.23.3
- use common method to pull etcd image
- prepare release v0.4.0-alpha.4
- implement reboot test
- enable slash commands in github PRs
- update bootkube
- update capi-upstream
- provide provisioned cluster info to integration test
- update bootkube fork
- rework firecracker code around upstream Go SDK + PRs
- **networkd:** Report on errors during interface configuration

### Docs

- add a link to the Talos Systems company site to the OSS site's header
- remove invalid field from docs
- **apid:** Add apid docs

### Feat

- **networkd:** Make healthcheck perform a check
- **networkd:** Add health api

### Fix

- bind etcd to IPv6 if available
- follow symlinks
- implement kubelet extra mounts
- **networkd:** fix ticker leak

### Test

- skip reboot tests

<a name="v0.4.0-alpha.3"></a>

## [v0.4.0-alpha.3](https://github.com/talos-systems/talos/compare/v0.4.0-alpha.2...v0.4.0-alpha.3) (2020-01-27)

### Chore

- prepare release v0.4.0-alpha.3
- refactor E2E scripts
- fix CI
- Clean up generated path for protoc
- use firecracker in basic-integration
- update bootkube config to include cluster name

### Docs

- fix machined component
- update metal section
- remove pre-release from v0.3 docs

### Feat

- update kernel
- allow ability to customize containerd
- allow for bootkube images to be customized

### Fix

- parse correctly kernel command line missing DNS config
- retry system disk busy check
- correctly split lines with /dev/kmsg output
- re-enable control plane flags
- leave etcd after draining node
- install sequence stuck on event bus

### Refactor

- use tls.Config as client credentials

### Test

- firecracker provisioner fixes, implement cluster destroy

<a name="v0.4.0-alpha.2"></a>

## [v0.4.0-alpha.2](https://github.com/talos-systems/talos/compare/v0.4.0-alpha.1...v0.4.0-alpha.2) (2020-01-20)

### Chore

- prepare release v0.4.0-alpha.2
- use v0.1.0 tools and pkgs
- run sonobuoy in quick mode
- validate installer image before upgrade
- bump tools/pkgs for Go 1.13.6
- remove test-framework
- log ignored ACPI events
- fix E2E script
- publish boot.tar.gz
- allow docgen to ignore a struct

### Docs

- add missing docs
- reorganize components sidebar and add ntpd

### Feat

- upgrade kubernetes version to 1.17.1
- allow additional manifests to be provided to bootkube
- upgrade Linux to v5.4.11
- upgrade Linux to v5.4.10
- add a basic architectural diagram and a call to action

### Fix

- block when handling bus event
- stop race condition between kubelet and networkd
- update networkd permissions
- check for installer image before proceeding with upgrade
- set kube-dns labels
- Update bootkube to include node ready check
- Ensure assets directory does not exist
- add Close func in remote generator
- refuse to upgrade if single master
- update kernel version constant
- shutdown on button/power ACPI event
- raise default NOFILE limit
- **networkd:** Set hostname properly for dhcp when no hostname option is returned
- **networkd:** Fix incorrect resolver settings

### Refactor

- use ConfiguratorBundle interface for config generate
- unify generate type and machine type
- use an interface for config data
- use config struct instead of string

### Test

- provision Talos clusters via Firecracker VMs

<a name="v0.4.0-alpha.1"></a>

## [v0.4.0-alpha.1](https://github.com/talos-systems/talos/compare/v0.4.0-alpha.0...v0.4.0-alpha.1) (2020-01-08)

### Chore

- prepare release v0.4.0-alpha.1
- disable iso artifact publication
- update all target in Makefile
- allow re-use of docker network for local clusters
- fix release dependency
- fix push events
- push latest tag on tag events
- use the correct condition for latest and edge pushes

### Feat

- enable DynamicKubeletConfiguration
- Upgrade bootkube
- support configurable docker-based clusters
- upgrade linux to v5.4.8

### Fix

- make the CNI URL error better
