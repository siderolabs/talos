
<a name="v0.4.0-rc.0"></a>
## [v0.4.0-rc.0](https://github.com/talos-systems/talos/compare/v0.5.0-alpha.0...v0.4.0-rc.0) (2020-04-14)

### Chore

* prepare release v0.4.0-rc.0
* update pkgs tag to v0.2.0
* address random CI nits
* prepare release v0.4.0-beta.1
* update sonobuoy to v0.18.0
* update timeout values for e2e tests
* prepare release v0.4.0-beta.0

### Feat

* upgrade Go to 1.14.2
* upgrade Linux to v5.5.15

### Fix

* resolve race condition in createNodes
* add hpsa drivers
* add bnx2 and bnx2x firmware
* wait for `system-containerd` to become healthy before proceeding
* mount TLS certs into bootkube container
* make sure Close() is called on every path

### Refactor

* don't log installer verification

### Test

* serialize `docs` step execution
* update versions used for upgrade tests


<a name="v0.5.0-alpha.0"></a>
## [v0.5.0-alpha.0](https://github.com/talos-systems/talos/compare/v0.4.0-beta.1...v0.5.0-alpha.0) (2020-04-08)

### Chore

* prepare release v0.5.0-alpha.0
* add PR template for contributors
* update sonobuoy to v0.18.0
* update timeout values for e2e tests

### Feat

* upgrade Linux to v5.5.15

### Fix

* add bnx2 and bnx2x firmware
* wait for `system-containerd` to become healthy before proceeding
* mount TLS certs into bootkube container
* make sure Close() is called on every path

### Test

* update versions used for upgrade tests
* serialize `docs` step execution


<a name="v0.4.0-beta.1"></a>
## [v0.4.0-beta.1](https://github.com/talos-systems/talos/compare/v0.4.0-beta.0...v0.4.0-beta.1) (2020-04-07)

### Chore

* prepare release v0.4.0-beta.1
* update sonobuoy to v0.18.0
* update timeout values for e2e tests

### Feat

* upgrade Linux to v5.5.15

### Fix

* add bnx2 and bnx2x firmware
* wait for `system-containerd` to become healthy before proceeding
* mount TLS certs into bootkube container
* make sure Close() is called on every path


<a name="v0.4.0-beta.0"></a>
## [v0.4.0-beta.0](https://github.com/talos-systems/talos/compare/v0.4.0-alpha.8...v0.4.0-beta.0) (2020-04-03)

### Chore

* prepare release v0.4.0-beta.0

### Docs

* Add example of a VLAN configured device.

### Feat

* add BNX drivers
* introduce ability to specify extra hosts in /etc/hosts
* allow for exposing ports on docker clusters
* move bootkube out as full service
* upgrade kubernetes to 1.18
* make `--wait` default option to `talosctl cluster create`

### Fix

* delete tag on revert with empty label
* move empty label check
* wait for USB storage
* ignore EINVAL on unmounting when mount point isn't mounted
* make upgrades work with UEFI
* don't use ARP table for networkd health check

### Refactor

* move Talos client package to `pkg/`
* include partition label when unmount fails

### Test

* mark long tests as !short


<a name="v0.4.0-alpha.8"></a>
## [v0.4.0-alpha.8](https://github.com/talos-systems/talos/compare/v0.3.3...v0.4.0-alpha.8) (2020-03-24)

### Chore

* prepare release v0.4.0-alpha.8
* update upgrade tests for new version, split into two tracks
* run npm audit fix
* prepare release v0.4.0-alpha.7
* fix formatting of imports
* update Firecracker Go SDK to the official release
* cleanup assets dir after bootkube is done
* improve handling of etcd responses in bootkube pre-func
* add service state to postfunc
* prepare release v0.4.0-alpha.6
* update pkgs & tools for Go 1.14
* fix small misprint
* push installer & talos images to the CI registry on every build
* move golangci-lint.yaml to .golangci.yml
* remove KubernetesVersion from provision request
* prepare release v0.4.0-alpha.5
* build app container images skipping export to host
* update pkgs
* support bootloader emulation in firecracker provisioner
* implement loadbalancer for firecracker provisioner
* prepare release v0.4.0-alpha.4
* sign .drone.yml
* only run ok-to-test when PR
* support slash commands in drone
* get correct drone status in github actions
* use upstream version of Firecracker Go SDK
* update golangci-lint-1.23.3
* use common method to pull etcd image
* pull in latest version of grpc-proxy
* implement reboot test
* enable slash commands in github PRs
* update bootkube
* update capi-upstream
* provide provisioned cluster info to integration test
* update bootkube fork
* rework firecracker code around upstream Go SDK + PRs
* remove Firecracker bridge interface in osctl cluster destroy
* prepare release v0.4.0-alpha.3
* refactor E2E scripts
* fix CI
* Clean up generated path for protoc
* use firecracker in basic-integration
* update bootkube config to include cluster name
* prepare release v0.4.0-alpha.2
* use v0.1.0 tools and pkgs
* run sonobuoy in quick mode
* validate installer image before upgrade
* bump tools/pkgs for Go 1.13.6
* remove test-framework
* log ignored ACPI events
* fix E2E script
* publish boot.tar.gz
* allow docgen to ignore a struct
* prepare release v0.4.0-alpha.1
* disable iso artifact publication
* update all target in Makefile
* allow re-use of docker network for local clusters
* fix release dependency
* fix push events
* push latest tag on tag events
* use the correct condition for latest and edge pushes
* prepare release v0.4.0-alpha.0
* fix releases
* use osctl cluster --wait in basic-integration
* exclude cron events in push-latest step
* fix conformance
* add more functions to the release script
* remove gitmeta references
* add help menu to the Makefile
* refactor Makefile to be more DRY
* use docker buildx
* fix KVM test
* **networkd:** Report on errors during interface configuration

### Docs

* add bare-metal install example yaml
* update the website generator's npm packages
* add a link to the Talos Systems company site to the OSS site's header
* reorganize components sidebar and add ntpd
* remove invalid field from docs
* fix machined component
* update metal section
* remove pre-release from v0.3 docs
* add missing docs
* **apid:** Add apid docs

### Feat

* update bootkube
* add usb storage support
* initial work for supporting vlans
* build talosctl for ARM v7
* build talosctl for ARM64
* rename osctl to talosctl
* add support for `--with-debug` to osctl cluster create
* split `osctl` commands into Talos API and cluster management
* upgrade Go to version 1.14.1
* update talos base packages
* add debug logs to networkd health check
* respect panic kernel flag
* allow for persistence of config data
* split routerd from apid
* make admin kubeconfig cert lifetime configurable
* add function for mounting a specific system disk partition
* generate kubeconfig on the fly on request
* support proxy in docker buildx
* support sending machine info
* add reboot flag to reset API
* implement registry mirror & config for image pull
* upgrade kubernetes version to 1.17.1
* osctl bash/zsh completion support
* update kernel
* allow ability to customize containerd
* allow for bootkube images to be customized
* support specifying CIDR for docker network
* allow additional manifests to be provided to bootkube
* upgrade Linux to v5.4.11
* upgrade Linux to v5.4.10
* add a basic architectural diagram and a call to action
* enable DynamicKubeletConfiguration
* Upgrade bootkube
* support configurable docker-based clusters
* upgrade linux to v5.4.8
* add installer command to installer container
* upgrade Linux to v5.4.5
* add support for tftp download
* humanize timestamp and size in `osctl list` output
* add support for tailing logs
* implement streaming mode of dmesg, parse messages
* **networkd:** Make healthcheck perform a check
* **networkd:** Add health api

### Fix

* update k8s to 1.17.3
* update rtnetlink checks for bit masks
* respect dns domain from machine config
* ensure printing of panic message
* add debug option to v1alpha1 config
* skip links without a carrier
* ensure hostname is never empty
* ensure CA cert generation respects the hour flag
* ensre proxy is used when fetching additional manifests for bootkube
* unmount bind mounts for system (fixes upgrade stuck on disk busy)
* refresh proxy settings from environment in image resolver
* default reboot flag to false
* add reboot flag to reset command
* stop firecracker launcher on signal
* fix reset command
* allow kublet to handle multiple service CIDRs
* validate install disk
* PodCIDR, ServiceCIDR should be comma sets
* don't proxy gRPC unix connections
* do not add empty netconf
* bind etcd to IPv6 if available
* use specified kubelet and etcd images
* follow symlinks
* implement kubelet extra mounts
* parse correctly kernel command line missing DNS config
* retry system disk busy check
* correctly split lines with /dev/kmsg output
* re-enable control plane flags
* leave etcd after draining node
* install sequence stuck on event bus
* fix output formats
* stop race condition between kubelet and networkd
* update networkd permissions
* use dash for default talos cluster name in docker
* issues discovered by lgtm tool
* Update bootkube to include node ready check
* Ensure assets directory does not exist
* add Close func in remote generator
* refuse to upgrade if single master
* update kernel version constant
* shutdown on button/power ACPI event
* set kube-dns labels
* check for installer image before proceeding with upgrade
* raise default NOFILE limit
* make the CNI URL error better
* set the correct kernel args for VMware
* fix error format
* use the correct mf file name
* update `osctl list` to report node name
* use the correct TLD for the container version label
* don't log `token` metadata field in grpc request log
* Reset default http client to work around proxyEnv
* block when handling bus event
* extend list of kmsg facilities
* fail on muliple nodes for commands which don't support it
* **machined:** Add additional defaults for http transport
* **networkd:** Make better route scoping decisions
* **networkd:** Set hostname properly for dhcp when no hostname option is returned
* **networkd:** Fix incorrect resolver settings
* **networkd:** fix ticker leak
* **networkd:** Check for IFF_RUNNING on link up

### Refactor

* perform upgrade upon reboot
* use go-procfs
* use tls.Config as client credentials
* use ConfiguratorBundle interface for config generate
* unify generate type and machine type
* use an interface for config data
* use config struct instead of string
* extract Talos cluster provisioner as common code
* osctl code cleanup, no functional changes
* use all validation code

### Test

* add test for empty hostname option
* add 'reset' integration test for Reset() API
* enable upgrade tests 0.4.x -> latest
* implement new class of tests: provision tests (upgrades)
* fix `RebootAllNodes` test to reboot all nodes in one call
* implement RebootAllNodes test
* skip reboot tests
* firecracker provisioner fixes, implement cluster destroy
* provision Talos clusters via Firecracker VMs
* ensure etcd is healthy on all control plane nodes
* add integration tests for (most) CLI commands

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


<a name="v0.3.3"></a>
## [v0.3.3](https://github.com/talos-systems/talos/compare/v0.4.0-alpha.7...v0.3.3) (2020-03-24)

### Chore

* prepare release v0.3.3
* use common method to pull etcd image
* update bootkube
* update bootkube fork
* don't export unit-tests-race
* prepare release v0.3.2
* upgrade packages to v0.1.1
* update bootkube config to include cluster name
* prepare release v0.3.1
* prepare release v0.3.0
* use v0.1.0 tools and pkgs
* run sonobuoy in quick mode
* validate installer image before upgrade
* bump tools/pkgs for Go 1.13.6
* prepare release v0.3.0-rc.0
* update all target in Makefile
* log ignored ACPI events
* fix E2E script
* publish boot.tar.gz
* disable iso artifact publication
* fix release dependency
* prepare release v0.3.0-beta.3
* fix push events
* push latest tag on tag events
* use the correct condition for latest and edge pushes
* fix releases
* prepare release v0.3.0-beta.3
* use osctl cluster --wait in basic-integration
* fix conformance
* exclude cron events in push-latest step
* remove gitmeta references
* add help menu to the Makefile
* refactor Makefile to be more DRY
* use docker buildx
* prepare release v0.3.0-beta.2
* pull in latest version of grpc-proxy
* prepare release v0.3.0-beta.1

### Docs

* run make docs
* remove invalid field from docs
* fix machined component
* update metal section
* remove pre-release from v0.3 docs
* update docs

### Feat

* allow additional manifests to be provided to bootkube
* upgrade kubernetes version to 1.17.1
* upgrade Linux to v5.4.11
* upgrade Linux to v5.4.10
* Upgrade bootkube
* upgrade linux to v5.4.8
* support specifying CIDR for docker network
* add installer command to installer container
* upgrade Linux to v5.4.5
* add support for tailing logs
* implement streaming mode of dmesg, parse messages
* humanize timestamp and size in `osctl list` output

### Fix

* don't proxy gRPC unix connections
* follow symlinks
* implement kubelet extra mounts
* update k8s to 1.17.3
* ignore link-local and other non routable addresses
* parse correctly kernel command line missing DNS config
* retry system disk busy check
* re-enable control plane flags
* leave etcd after draining node
* install sequence stuck on event bus
* Ensure assets directory does not exist
* Update bootkube to include node ready check
* block when handling bus event
* stop race condition between kubelet and networkd
* update networkd permissions
* fail on muliple nodes for commands which don't support it
* issues discovered by lgtm tool
* add Close func in remote generator
* refuse to upgrade if single master
* Reset default http client to work around proxyEnv
* shutdown on button/power ACPI event
* set kube-dns labels
* check for installer image before proceeding with upgrade
* raise default NOFILE limit
* make the CNI URL error better
* set the correct kernel args for VMware
* use the correct mf file name
* use the correct TLD for the container version label
* update `osctl list` to report node name
* don't log `token` metadata field in grpc request log
* extend list of kmsg facilities
* update kernel version constant
* fix output formats
* use specified kubelet and etcd images
* **machined:** Add additional defaults for http transport
* **networkd:** Make better route scoping decisions
* **networkd:** Set hostname properly for dhcp when no hostname option is returned
* **networkd:** Fix incorrect resolver settings
* **networkd:** Check for IFF_RUNNING on link up

### Refactor

* use all validation code
* extract Talos cluster provisioner as common code
* osctl code cleanup, no functional changes

### Test

* ensure etcd is healthy on all control plane nodes
* add integration tests for (most) CLI commands


<a name="v0.4.0-alpha.7"></a>
## [v0.4.0-alpha.7](https://github.com/talos-systems/talos/compare/v0.4.0-alpha.6...v0.4.0-alpha.7) (2020-03-20)

### Chore

* prepare release v0.4.0-alpha.7
* fix formatting of imports
* update Firecracker Go SDK to the official release
* cleanup assets dir after bootkube is done
* improve handling of etcd responses in bootkube pre-func
* add service state to postfunc

### Docs

* update the website generator's npm packages

### Feat

* rename osctl to talosctl
* add support for `--with-debug` to osctl cluster create
* split `osctl` commands into Talos API and cluster management
* upgrade Go to version 1.14.1
* update talos base packages
* add debug logs to networkd health check
* respect panic kernel flag
* allow for persistence of config data
* split routerd from apid
* make admin kubeconfig cert lifetime configurable
* add function for mounting a specific system disk partition
* generate kubeconfig on the fly on request

### Fix

* respect dns domain from machine config
* ensure printing of panic message
* add debug option to v1alpha1 config
* skip links without a carrier
* ensure hostname is never empty
* ensure CA cert generation respects the hour flag

### Refactor

* perform upgrade upon reboot

### Test

* add test for empty hostname option
* add 'reset' integration test for Reset() API

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

* prepare release v0.4.0-alpha.6
* update pkgs & tools for Go 1.14
* fix small misprint
* push installer & talos images to the CI registry on every build
* move golangci-lint.yaml to .golangci.yml
* remove KubernetesVersion from provision request

### Feat

* support proxy in docker buildx
* support sending machine info
* add reboot flag to reset API

### Fix

* ensre proxy is used when fetching additional manifests for bootkube
* unmount bind mounts for system (fixes upgrade stuck on disk busy)
* refresh proxy settings from environment in image resolver
* default reboot flag to false
* add reboot flag to reset command
* stop firecracker launcher on signal
* fix reset command
* allow kublet to handle multiple service CIDRs
* validate install disk
* PodCIDR, ServiceCIDR should be comma sets

### Refactor

* use go-procfs

### Test

* enable upgrade tests 0.4.x -> latest
* implement new class of tests: provision tests (upgrades)
* fix `RebootAllNodes` test to reboot all nodes in one call
* implement RebootAllNodes test


<a name="v0.4.0-alpha.5"></a>
## [v0.4.0-alpha.5](https://github.com/talos-systems/talos/compare/v0.4.0-alpha.4...v0.4.0-alpha.5) (2020-02-15)

### Chore

* prepare release v0.4.0-alpha.5
* build app container images skipping export to host
* update pkgs
* support bootloader emulation in firecracker provisioner
* implement loadbalancer for firecracker provisioner

### Feat

* implement registry mirror & config for image pull

### Fix

* don't proxy gRPC unix connections
* do not add empty netconf


<a name="v0.4.0-alpha.4"></a>
## [v0.4.0-alpha.4](https://github.com/talos-systems/talos/compare/v0.4.0-alpha.3...v0.4.0-alpha.4) (2020-02-04)

### Chore

* remove Firecracker bridge interface in osctl cluster destroy
* sign .drone.yml
* only run ok-to-test when PR
* support slash commands in drone
* get correct drone status in github actions
* use upstream version of Firecracker Go SDK
* update golangci-lint-1.23.3
* use common method to pull etcd image
* prepare release v0.4.0-alpha.4
* implement reboot test
* enable slash commands in github PRs
* update bootkube
* update capi-upstream
* provide provisioned cluster info to integration test
* update bootkube fork
* rework firecracker code around upstream Go SDK + PRs
* **networkd:** Report on errors during interface configuration

### Docs

* add a link to the Talos Systems company site to the OSS site's header
* remove invalid field from docs
* **apid:** Add apid docs

### Feat

* **networkd:** Make healthcheck perform a check
* **networkd:** Add health api

### Fix

* bind etcd to IPv6 if available
* follow symlinks
* implement kubelet extra mounts
* **networkd:** fix ticker leak

### Test

* skip reboot tests


<a name="v0.4.0-alpha.3"></a>
## [v0.4.0-alpha.3](https://github.com/talos-systems/talos/compare/v0.3.2...v0.4.0-alpha.3) (2020-01-27)

### Chore

* prepare release v0.4.0-alpha.3
* refactor E2E scripts
* fix CI
* Clean up generated path for protoc
* use firecracker in basic-integration
* update bootkube config to include cluster name
* prepare release v0.4.0-alpha.2
* use v0.1.0 tools and pkgs
* run sonobuoy in quick mode
* validate installer image before upgrade
* bump tools/pkgs for Go 1.13.6
* remove test-framework
* log ignored ACPI events
* fix E2E script
* publish boot.tar.gz
* allow docgen to ignore a struct
* prepare release v0.4.0-alpha.1
* disable iso artifact publication
* update all target in Makefile
* allow re-use of docker network for local clusters
* fix release dependency
* fix push events
* push latest tag on tag events
* use the correct condition for latest and edge pushes
* prepare release v0.4.0-alpha.0
* fix releases
* use osctl cluster --wait in basic-integration
* exclude cron events in push-latest step
* fix conformance
* add more functions to the release script
* remove gitmeta references
* add help menu to the Makefile
* refactor Makefile to be more DRY
* use docker buildx
* pull in latest version of grpc-proxy
* fix KVM test

### Docs

* fix machined component
* update metal section
* remove pre-release from v0.3 docs
* add missing docs
* reorganize components sidebar and add ntpd

### Feat

* update kernel
* allow ability to customize containerd
* allow for bootkube images to be customized
* upgrade kubernetes version to 1.17.1
* allow additional manifests to be provided to bootkube
* upgrade Linux to v5.4.11
* upgrade Linux to v5.4.10
* add a basic architectural diagram and a call to action
* enable DynamicKubeletConfiguration
* Upgrade bootkube
* support configurable docker-based clusters
* upgrade linux to v5.4.8
* add installer command to installer container
* upgrade Linux to v5.4.5
* add support for tftp download
* humanize timestamp and size in `osctl list` output
* add support for tailing logs
* support specifying CIDR for docker network
* osctl bash/zsh completion support
* implement streaming mode of dmesg, parse messages

### Fix

* parse correctly kernel command line missing DNS config
* retry system disk busy check
* correctly split lines with /dev/kmsg output
* re-enable control plane flags
* leave etcd after draining node
* install sequence stuck on event bus
* block when handling bus event
* stop race condition between kubelet and networkd
* update networkd permissions
* use specified kubelet and etcd images
* use dash for default talos cluster name in docker
* Update bootkube to include node ready check
* Ensure assets directory does not exist
* add Close func in remote generator
* refuse to upgrade if single master
* update kernel version constant
* shutdown on button/power ACPI event
* set kube-dns labels
* extend list of kmsg facilities
* raise default NOFILE limit
* make the CNI URL error better
* set the correct kernel args for VMware
* fix error format
* use the correct mf file name
* issues discovered by lgtm tool
* use the correct TLD for the container version label
* don't log `token` metadata field in grpc request log
* update `osctl list` to report node name
* fix output formats
* check for installer image before proceeding with upgrade
* Reset default http client to work around proxyEnv
* fail on muliple nodes for commands which don't support it
* **machined:** Add additional defaults for http transport
* **networkd:** Make better route scoping decisions
* **networkd:** Set hostname properly for dhcp when no hostname option is returned
* **networkd:** Fix incorrect resolver settings
* **networkd:** Check for IFF_RUNNING on link up

### Refactor

* use tls.Config as client credentials
* use ConfiguratorBundle interface for config generate
* unify generate type and machine type
* use an interface for config data
* use config struct instead of string
* extract Talos cluster provisioner as common code
* osctl code cleanup, no functional changes
* use all validation code

### Test

* firecracker provisioner fixes, implement cluster destroy
* provision Talos clusters via Firecracker VMs
* ensure etcd is healthy on all control plane nodes
* add integration tests for (most) CLI commands


<a name="v0.3.2"></a>
## [v0.3.2](https://github.com/talos-systems/talos/compare/v0.3.1...v0.3.2) (2020-01-27)

### Chore

* don't export unit-tests-race
* prepare release v0.3.2
* upgrade packages to v0.1.1
* update bootkube config to include cluster name

### Docs

* fix machined component
* update metal section
* remove pre-release from v0.3 docs

### Fix

* parse correctly kernel command line missing DNS config
* retry system disk busy check
* re-enable control plane flags


<a name="v0.3.1"></a>
## [v0.3.1](https://github.com/talos-systems/talos/compare/v0.3.0...v0.3.1) (2020-01-21)

### Chore

* prepare release v0.3.1

### Fix

* leave etcd after draining node
* install sequence stuck on event bus


<a name="v0.3.0"></a>
## [v0.3.0](https://github.com/talos-systems/talos/compare/v0.4.0-alpha.2...v0.3.0) (2020-01-21)

### Chore

* prepare release v0.3.0
* use v0.1.0 tools and pkgs
* run sonobuoy in quick mode
* validate installer image before upgrade
* bump tools/pkgs for Go 1.13.6
* prepare release v0.3.0-rc.0
* update all target in Makefile
* log ignored ACPI events
* fix E2E script
* publish boot.tar.gz
* disable iso artifact publication
* fix release dependency
* prepare release v0.3.0-beta.3
* fix push events
* push latest tag on tag events
* use the correct condition for latest and edge pushes
* fix releases
* prepare release v0.3.0-beta.3
* use osctl cluster --wait in basic-integration
* fix conformance
* exclude cron events in push-latest step
* remove gitmeta references
* add help menu to the Makefile
* refactor Makefile to be more DRY
* use docker buildx
* prepare release v0.3.0-beta.2
* pull in latest version of grpc-proxy
* prepare release v0.3.0-beta.1

### Docs

* update docs

### Feat

* allow additional manifests to be provided to bootkube
* upgrade kubernetes version to 1.17.1
* upgrade Linux to v5.4.11
* upgrade Linux to v5.4.10
* Upgrade bootkube
* upgrade linux to v5.4.8
* support specifying CIDR for docker network
* add installer command to installer container
* upgrade Linux to v5.4.5
* add support for tailing logs
* implement streaming mode of dmesg, parse messages
* humanize timestamp and size in `osctl list` output

### Fix

* Ensure assets directory does not exist
* Update bootkube to include node ready check
* block when handling bus event
* stop race condition between kubelet and networkd
* update networkd permissions
* fail on muliple nodes for commands which don't support it
* issues discovered by lgtm tool
* add Close func in remote generator
* refuse to upgrade if single master
* update kernel version constant
* shutdown on button/power ACPI event
* set kube-dns labels
* check for installer image before proceeding with upgrade
* raise default NOFILE limit
* Reset default http client to work around proxyEnv
* set the correct kernel args for VMware
* use the correct mf file name
* use the correct TLD for the container version label
* update `osctl list` to report node name
* don't log `token` metadata field in grpc request log
* extend list of kmsg facilities
* make the CNI URL error better
* fix output formats
* use specified kubelet and etcd images
* **machined:** Add additional defaults for http transport
* **networkd:** Make better route scoping decisions
* **networkd:** Set hostname properly for dhcp when no hostname option is returned
* **networkd:** Fix incorrect resolver settings
* **networkd:** Check for IFF_RUNNING on link up

### Refactor

* use all validation code
* extract Talos cluster provisioner as common code
* osctl code cleanup, no functional changes

### Test

* ensure etcd is healthy on all control plane nodes
* add integration tests for (most) CLI commands


<a name="v0.4.0-alpha.2"></a>
## [v0.4.0-alpha.2](https://github.com/talos-systems/talos/compare/v0.3.0-rc.0...v0.4.0-alpha.2) (2020-01-20)

### Chore

* prepare release v0.4.0-alpha.2
* use v0.1.0 tools and pkgs
* run sonobuoy in quick mode
* validate installer image before upgrade
* bump tools/pkgs for Go 1.13.6
* remove test-framework
* log ignored ACPI events
* fix E2E script
* publish boot.tar.gz
* allow docgen to ignore a struct
* prepare release v0.4.0-alpha.1
* disable iso artifact publication
* update all target in Makefile
* allow re-use of docker network for local clusters
* fix release dependency
* fix push events
* push latest tag on tag events
* use the correct condition for latest and edge pushes
* prepare release v0.4.0-alpha.0
* fix releases
* use osctl cluster --wait in basic-integration
* exclude cron events in push-latest step
* fix conformance
* add more functions to the release script
* remove gitmeta references
* add help menu to the Makefile
* refactor Makefile to be more DRY
* use docker buildx
* pull in latest version of grpc-proxy
* fix KVM test

### Docs

* add missing docs
* reorganize components sidebar and add ntpd

### Feat

* upgrade kubernetes version to 1.17.1
* allow additional manifests to be provided to bootkube
* upgrade Linux to v5.4.11
* upgrade Linux to v5.4.10
* add a basic architectural diagram and a call to action
* enable DynamicKubeletConfiguration
* Upgrade bootkube
* support configurable docker-based clusters
* upgrade linux to v5.4.8
* add installer command to installer container
* upgrade Linux to v5.4.5
* add support for tftp download
* humanize timestamp and size in `osctl list` output
* add support for tailing logs
* support specifying CIDR for docker network
* osctl bash/zsh completion support
* implement streaming mode of dmesg, parse messages

### Fix

* block when handling bus event
* stop race condition between kubelet and networkd
* update networkd permissions
* use specified kubelet and etcd images
* use dash for default talos cluster name in docker
* Update bootkube to include node ready check
* Ensure assets directory does not exist
* add Close func in remote generator
* refuse to upgrade if single master
* update kernel version constant
* shutdown on button/power ACPI event
* set kube-dns labels
* check for installer image before proceeding with upgrade
* raise default NOFILE limit
* make the CNI URL error better
* extend list of kmsg facilities
* fix error format
* use the correct mf file name
* issues discovered by lgtm tool
* use the correct TLD for the container version label
* don't log `token` metadata field in grpc request log
* update `osctl list` to report node name
* fix output formats
* set the correct kernel args for VMware
* Reset default http client to work around proxyEnv
* fail on muliple nodes for commands which don't support it
* **machined:** Add additional defaults for http transport
* **networkd:** Make better route scoping decisions
* **networkd:** Set hostname properly for dhcp when no hostname option is returned
* **networkd:** Fix incorrect resolver settings
* **networkd:** Check for IFF_RUNNING on link up

### Refactor

* use ConfiguratorBundle interface for config generate
* unify generate type and machine type
* use an interface for config data
* use config struct instead of string
* extract Talos cluster provisioner as common code
* osctl code cleanup, no functional changes
* use all validation code

### Test

* provision Talos clusters via Firecracker VMs
* ensure etcd is healthy on all control plane nodes
* add integration tests for (most) CLI commands


<a name="v0.3.0-rc.0"></a>
## [v0.3.0-rc.0](https://github.com/talos-systems/talos/compare/v0.4.0-alpha.1...v0.3.0-rc.0) (2020-01-11)

### Chore

* prepare release v0.3.0-rc.0
* update all target in Makefile
* log ignored ACPI events
* fix E2E script
* publish boot.tar.gz
* disable iso artifact publication
* fix release dependency
* prepare release v0.3.0-beta.3
* fix push events
* push latest tag on tag events
* use the correct condition for latest and edge pushes
* fix releases
* prepare release v0.3.0-beta.3
* use osctl cluster --wait in basic-integration
* fix conformance
* exclude cron events in push-latest step
* remove gitmeta references
* add help menu to the Makefile
* refactor Makefile to be more DRY
* use docker buildx
* prepare release v0.3.0-beta.2
* pull in latest version of grpc-proxy
* prepare release v0.3.0-beta.1

### Docs

* update docs

### Feat

* upgrade Linux to v5.4.10
* Upgrade bootkube
* upgrade linux to v5.4.8
* support specifying CIDR for docker network
* add installer command to installer container
* upgrade Linux to v5.4.5
* add support for tailing logs
* implement streaming mode of dmesg, parse messages
* humanize timestamp and size in `osctl list` output

### Fix

* shutdown on button/power ACPI event
* set kube-dns labels
* check for installer image before proceeding with upgrade
* raise default NOFILE limit
* make the CNI URL error better
* set the correct kernel args for VMware
* use the correct mf file name
* use the correct TLD for the container version label
* fail on muliple nodes for commands which don't support it
* don't log `token` metadata field in grpc request log
* extend list of kmsg facilities
* issues discovered by lgtm tool
* fix output formats
* update `osctl list` to report node name
* Reset default http client to work around proxyEnv
* use specified kubelet and etcd images
* **machined:** Add additional defaults for http transport
* **networkd:** Check for IFF_RUNNING on link up
* **networkd:** Make better route scoping decisions

### Refactor

* use all validation code
* extract Talos cluster provisioner as common code
* osctl code cleanup, no functional changes

### Test

* ensure etcd is healthy on all control plane nodes
* add integration tests for (most) CLI commands


<a name="v0.4.0-alpha.1"></a>
## [v0.4.0-alpha.1](https://github.com/talos-systems/talos/compare/v0.3.0-beta.3...v0.4.0-alpha.1) (2020-01-08)

### Chore

* prepare release v0.4.0-alpha.1
* disable iso artifact publication
* update all target in Makefile
* allow re-use of docker network for local clusters
* fix release dependency
* fix push events
* push latest tag on tag events
* use the correct condition for latest and edge pushes
* prepare release v0.4.0-alpha.0
* fix releases
* use osctl cluster --wait in basic-integration
* exclude cron events in push-latest step
* fix conformance
* add more functions to the release script
* remove gitmeta references
* add help menu to the Makefile
* refactor Makefile to be more DRY
* use docker buildx
* pull in latest version of grpc-proxy
* fix KVM test

### Feat

* enable DynamicKubeletConfiguration
* Upgrade bootkube
* support configurable docker-based clusters
* upgrade linux to v5.4.8
* add installer command to installer container
* upgrade Linux to v5.4.5
* add support for tftp download
* humanize timestamp and size in `osctl list` output
* add support for tailing logs
* support specifying CIDR for docker network
* osctl bash/zsh completion support
* implement streaming mode of dmesg, parse messages

### Fix

* make the CNI URL error better
* set the correct kernel args for VMware
* fix error format
* use the correct mf file name
* use specified kubelet and etcd images
* use the correct TLD for the container version label
* don't log `token` metadata field in grpc request log
* use dash for default talos cluster name in docker
* fix output formats
* extend list of kmsg facilities
* issues discovered by lgtm tool
* Reset default http client to work around proxyEnv
* update `osctl list` to report node name
* fail on muliple nodes for commands which don't support it
* **machined:** Add additional defaults for http transport
* **networkd:** Check for IFF_RUNNING on link up
* **networkd:** Make better route scoping decisions

### Refactor

* extract Talos cluster provisioner as common code
* osctl code cleanup, no functional changes
* use all validation code

### Test

* ensure etcd is healthy on all control plane nodes
* add integration tests for (most) CLI commands


<a name="v0.3.0-beta.3"></a>
## [v0.3.0-beta.3](https://github.com/talos-systems/talos/compare/v0.4.0-alpha.0...v0.3.0-beta.3) (2020-01-01)

### Chore

* prepare release v0.3.0-beta.3
* fix push events
* push latest tag on tag events
* use the correct condition for latest and edge pushes
* fix releases
* prepare release v0.3.0-beta.3
* use osctl cluster --wait in basic-integration
* fix conformance
* exclude cron events in push-latest step
* remove gitmeta references
* add help menu to the Makefile
* refactor Makefile to be more DRY
* use docker buildx
* prepare release v0.3.0-beta.2
* pull in latest version of grpc-proxy
* prepare release v0.3.0-beta.1

### Docs

* update docs

### Feat

* support specifying CIDR for docker network
* add installer command to installer container
* upgrade Linux to v5.4.5
* add support for tailing logs
* implement streaming mode of dmesg, parse messages
* humanize timestamp and size in `osctl list` output

### Fix

* set the correct kernel args for VMware
* use the correct mf file name
* use the correct TLD for the container version label
* fail on muliple nodes for commands which don't support it
* don't log `token` metadata field in grpc request log
* extend list of kmsg facilities
* issues discovered by lgtm tool
* fix output formats
* update `osctl list` to report node name
* Reset default http client to work around proxyEnv
* use specified kubelet and etcd images
* **machined:** Add additional defaults for http transport
* **networkd:** Check for IFF_RUNNING on link up
* **networkd:** Make better route scoping decisions

### Refactor

* extract Talos cluster provisioner as common code
* osctl code cleanup, no functional changes

### Test

* ensure etcd is healthy on all control plane nodes
* add integration tests for (most) CLI commands


<a name="v0.4.0-alpha.0"></a>
## [v0.4.0-alpha.0](https://github.com/talos-systems/talos/compare/v0.3.0-beta.2...v0.4.0-alpha.0) (2020-01-01)

### Chore

* prepare release v0.4.0-alpha.0
* fix releases
* use osctl cluster --wait in basic-integration
* exclude cron events in push-latest step
* fix conformance
* add more functions to the release script
* remove gitmeta references
* add help menu to the Makefile
* refactor Makefile to be more DRY
* use docker buildx
* pull in latest version of grpc-proxy
* fix KVM test

### Feat

* add installer command to installer container
* upgrade Linux to v5.4.5
* add support for tftp download
* humanize timestamp and size in `osctl list` output
* add support for tailing logs
* support specifying CIDR for docker network
* osctl bash/zsh completion support
* implement streaming mode of dmesg, parse messages

### Fix

* set the correct kernel args for VMware
* fix error format
* use the correct mf file name
* use specified kubelet and etcd images
* use the correct TLD for the container version label
* don't log `token` metadata field in grpc request log
* use dash for default talos cluster name in docker
* fix output formats
* extend list of kmsg facilities
* issues discovered by lgtm tool
* Reset default http client to work around proxyEnv
* update `osctl list` to report node name
* fail on muliple nodes for commands which don't support it
* **machined:** Add additional defaults for http transport
* **networkd:** Check for IFF_RUNNING on link up
* **networkd:** Make better route scoping decisions

### Refactor

* extract Talos cluster provisioner as common code
* osctl code cleanup, no functional changes
* use all validation code

### Test

* ensure etcd is healthy on all control plane nodes
* add integration tests for (most) CLI commands


<a name="v0.3.0-beta.2"></a>
## [v0.3.0-beta.2](https://github.com/talos-systems/talos/compare/v0.3.0-beta.1...v0.3.0-beta.2) (2019-12-21)

### Chore

* prepare release v0.3.0-beta.2
* pull in latest version of grpc-proxy

### Feat

* upgrade Linux to v5.4.5
* add support for tailing logs

### Fix

* Reset default http client to work around proxyEnv
* **networkd:** Check for IFF_RUNNING on link up

### Refactor

* osctl code cleanup, no functional changes

### Test

* add integration tests for (most) CLI commands


<a name="v0.3.0-beta.1"></a>
## [v0.3.0-beta.1](https://github.com/talos-systems/talos/compare/v0.3.0-beta.0...v0.3.0-beta.1) (2019-12-19)

### Chore

* prepare release v0.3.0-beta.1

### Feat

* implement streaming mode of dmesg, parse messages
* humanize timestamp and size in `osctl list` output

### Fix

* update `osctl list` to report node name
* issues discovered by lgtm tool
* fail on muliple nodes for commands which don't support it
* use specified kubelet and etcd images


<a name="v0.3.0-beta.0"></a>
## [v0.3.0-beta.0](https://github.com/talos-systems/talos/compare/v0.3.0-alpha.10...v0.3.0-beta.0) (2019-12-11)

### Chore

* prepare release v0.3.0-beta.0
* update client-go
* make the CNI configurable in local KVM test
* Remove increased timeouts for dhcp addressing.
* Add link name to dhcp addressing error
* upgrade sonobuoy to v0.16.5
* update gcp disk sizes
* update containerd client version
* rewrite basic integration in go instead of bash
* support image specification in drone step function
* validate url input for osctl config generate

### Docs

* update osctl kubeconfig references
* simplify corporate proxy
* update generated osctl documentation
* update with new cni abilities
* clarify vmware instructions

### Feat

* add create and overwrite file operations
* add config nodes command
* Upgrade kubernetes to 1.17.0
* add security hardening settings
* rename confusing target options, --endpoints, etc.
* make osd.Dmesg API streaming
* add domain search line to resolv.conf
* allow configurable SANs for API
* allow ability to specify custom CNIs
* add support for `osctl logs -f`
* add ability to append to existing files with extrafiles
* upgrade Linux to v5.3.15
* add universal TUN/TAP device driver support
* use containerd-shim-runc-v2
* upgrade containerd to v1.3.2
* osctl logs now supports multiple targets
* support output directory for osctl config generate
* support client only version for osctl
* allow deep-linking to specific docs pages

### Fix

* mount as rshared
* allow initial-cluster-state to be set
* kill POD network mode pods first on upgrades
* improve the project site meta description
* response filtering for client API, RunE for osctl
* update node dependencies for project website
* append domainname to DHCP-sourced hostname
* strip line feed from domainname after read
* don't set br_netfilter sysctls in container mode
* add missing sysctl params required by containerd
* reverse preference order of network config
* don't use netrc
* run go mod tidy
* error reporting in `osctl kubeconfig`
* make retry errors ordered
* return a unique set of errors on retry failure
* mount /run as shared in container mode
* close io.ReadCloser
* Add hostname setting to networkd
* extract errors from API response
* update kernel version constant
* provide peer remote address for 'NODE': as default in osctl
* **networkd:** Ignore loopback interface during hostname decision.

### Refactor

* rename protobuf services, RPCs, and messages
* update --image shorthand flag to -i
* extract TLS bits from apid main.go
* set CRI config to /etc/cri/containerd.toml
* simplify NewTemporaryClientFromPKI
* **networkd:** Update bond config parameters to align with kernel

### Test

* add retries to the test which verifies cluster version
* fix flakey test on linear retries
* fix integration version test as 'NODE:' might be missing


<a name="v0.3.0-alpha.10"></a>
## [v0.3.0-alpha.10](https://github.com/talos-systems/talos/compare/v0.3.0-alpha.9...v0.3.0-alpha.10) (2019-12-02)

### Chore

* prepare release v0.3.0-alpha.10
* upgrade packages
* fix conformance
* update bootkube
* upgrade sonobuoy
* push edge tag on succesful conformance
* add ability to specify custom intaller to libvirt setup
* **networkd:** Ignore bonded interfaces without config

### Docs

* add automated upgrades proposal
* fix documentation link

### Feat

* enable aggregation layer
* support force flag for osctl kubeconfig
* enable webhook authorization mode
* use grpc-proxy in apid
* upgrade packages
* add Google Analytics tracking to the project website
* add IMA policy
* enable IMA measurement and appraisal
* **networkd:** Add support for kernel nfsroot arguments.
* **networkd:** Add support for bonding

### Fix

* update kernel version constant
* osctl panic when metadata is nil
* prevent nil pointer panic
* provide a way for client TLS config to use Provider
* recover control plane on reboot
* ensure etcd comes back up on reboot of all members
* require mode flag when validating
* don't measure overlayfs

### Refactor

* rename virtual package to pseudo


<a name="v0.3.0-alpha.9"></a>
## [v0.3.0-alpha.9](https://github.com/talos-systems/talos/compare/v0.3.0-alpha.8...v0.3.0-alpha.9) (2019-11-25)

### Chore

* prepare release v0.3.0-alpha.9
* disable all azure e2e temporarily
* Fix formatting ( make fmt )

### Docs

* add matchbox getting started guide

### Feat

* add read API
* allow sysctl writes
* upgrade packages

### Fix

* retry cordon and uncordon
* require arg length of 1 for kubeconfig command

### Refactor

* rename version label
* remove CNI bundle


<a name="v0.3.0-alpha.8"></a>
## [v0.3.0-alpha.8](https://github.com/talos-systems/talos/compare/v0.3.0-alpha.7...v0.3.0-alpha.8) (2019-11-15)

### Chore

* prepare release v0.3.0-alpha.8
* reverse order of events in `osctl service`
* address deprecation warning from netlink package

### Docs

* Add examples to networkd
* update gcp docs
* Update azure doc
* add docs command to osctl
* add a project dropdown
* remove stale docs
* fix proxy Dockerfile example
* disable PurgeCSS

### Feat

* allow extra arguments to be passed to etcd
* Add context key to osctl
* Add support for resetting the network during bootup

### Fix

* set --upgrade flag properly on installs
* honor the extraArgs option for the kubelet
* make logging middleware first in the list, fix duration
* use the config's cluster version for control plane image

### Refactor

* rename initial network task func

### Test

* disable discovery-based test as it might break e2e
* add integration test for full boot sequence
* implement node discovery for integration tests


<a name="v0.3.0-alpha.7"></a>
## [v0.3.0-alpha.7](https://github.com/talos-systems/talos/compare/v0.3.0-alpha.6...v0.3.0-alpha.7) (2019-11-12)

### Chore

* prepare release v0.3.0-alpha.7
* format docs to one sentence per line
* remove CertificateKey
* Move back to official procfs repo
* run gofumports after protoc-gen
* re-enable e2e for aws clusters
* add simple health check for etcd service
* re-enable e2e testing

### Docs

* add customization guide for running behing a proxy
* add autogenerated config reference
* fix roadmap layout
* update landing page
* add public roadmap
* Add machine.env section

### Feat

* implement grpc request loggging
* Add support for defining ntp servers via config
* Add meminfo api
* Disable networkd configuration if `ip` kernel parameter is specified
* Add support for streaming apis in apid
* Add support for setting container output to stdout
* add metadata file to boot partition
* **networkd:** Add support for custom nameservers

### Fix

* upgrade rtnetlink package
* mount extra disks after system disk
* remove duplicate line
* recover from panics in grpc servers
* pass x509 options to NewCertificateFromCSR
* remove global variable in bootkube
* conditionally create a new etcd cluster

### Test

* fix integration test for k8s version
* add 'integration-test' to e2e runs
* add k8s integration tests
* add CLI integration test


<a name="v0.3.0-alpha.6"></a>
## [v0.3.0-alpha.6](https://github.com/talos-systems/talos/compare/v0.3.0-alpha.5...v0.3.0-alpha.6) (2019-11-05)

### Chore

* prepare release v0.3.0-alpha.6
* remove bind mounts from OSD
* Move data messages to common proto
* install customization requirements with ONBUILD
* force overwrite of output file
* remove unused files

### Docs

* various layout and responsiveness fixes

### Feat

* output machined logs to /dev/kmsg and file
* add timestamp to installed file
* create cluster with default PSP

### Fix

* Disable support for proxy variables for apid.
* sleep in NTP query loop
* Add host network namespace to networkd and ntpd
* verify that all etcd members are running before upgrading
* don't use 127.0.0.1 for etcd client
* add etcd member conditionally
* stop etcd and remove data-dir
* use CRI to stop containers
* verify system disk not in use
* retry BLKPG operations
* stop leaking file descriptors
* send SIGKILL to hanging containers
* be explicit about installs
* Avoid running bootkube on reboots
* **osd:** Add additional capabilities for osd

### Refactor

* rename Helper to Client
* Move logs to machined
* Move kubeconfig to machined
* use retry package in ntpd
* unify service stop on upgrade
* use constants.SystemContainerdNamespace
* pass runtime to initializer
* align platform names with kernel args
* use etcd package

### Test

* add integration test framework


<a name="v0.3.0-alpha.5"></a>
## [v0.3.0-alpha.5](https://github.com/talos-systems/talos/compare/v0.3.0-alpha.4...v0.3.0-alpha.5) (2019-10-31)

### Chore

* prepare release v0.3.0-alpha.5
* remove RAW disk
* update pkgs SHA

### Docs

* remove v0.2 docs
* fix list-style-position
* add customization guide
* add VMware docs to menu
* add troubleshooting guide on common PKI scenarios
* add note on CRNG initialization

### Feat

* Add support for creating VMware images
* use Ed25519 public-key signature system
* upgrade Kubernetes to 1.16.2
* lock down container permissions

### Fix

* check if endpoint is nil


<a name="v0.3.0-alpha.4"></a>
## [v0.3.0-alpha.4](https://github.com/talos-systems/talos/compare/v0.3.0-alpha.3...v0.3.0-alpha.4) (2019-10-28)

### Chore

* prepare release v0.3.0-alpha.4
* add Digital Ocean image to release
* replace `/* */` comments with `//` comments in license header
* bump tools/pkgs for toolchain refactor
* fix markdown lint error

### Docs

* fix Digital Ocean docs
* more whitespace, wording, and responsiveness changes

### Feat

* add support for Digital Ocean
* Add retry on get kubeconfig
* Add network api to apid
* Add time api to apid
* Add APId

### Fix

* add cluster endpoint to certificate SANs
* Fix osctl version output


<a name="v0.3.0-alpha.3"></a>
## [v0.3.0-alpha.3](https://github.com/talos-systems/talos/compare/v0.3.0-alpha.2...v0.3.0-alpha.3) (2019-10-25)

### Chore

* prepare release v0.3.0-alpha.3
* attempt to avoid containerd shim socket conflicts in tests
* attempt to fix test hanging with reaper enabled
* fix containerd test hanging
* make service_runner_test less flaky
* fix flaky constant retry test
* make Slack notifications more fancy
* run 'git fetch --tags' as first step
* Rename proto files into more appropriate names

### Docs

* responsiveness fixes and wording changes
* update getting started guide
* add v0.3 AWS guide
* improve asciinema casts
* remove header animation
* make the footer bigger

### Feat

* detect gzipped machine configs
* Add node metadata wrapper to machine api

### Fix

* append localhost to cert sans if docker platform
* create external IP failures as non-fatal
* ensure control plane endpoint is set


<a name="v0.3.0-alpha.2"></a>
## [v0.3.0-alpha.2](https://github.com/talos-systems/talos/compare/v0.3.0-alpha.1...v0.3.0-alpha.2) (2019-10-21)

### Chore

* prepare release v0.3.0-alpha.2
* bump tools & pkgs for Go 1.13.2
* Update gitmeta to latest release
* bump golangci-lint to 1.21
* remove custom log paths

### Docs

* improve landing page terminal
* add ephemeral feature note
* add API examples to the landing page
* improve landing page
* make the sidebar sticky
* change doc content margins and padding
* move docs version dropdown to docs page
* use horizontal containerd logo
* add FAQs page
* add community dropdown
* improve dropdown menu
* show background only on landing page
* add landing page
* add v0.3 boilerplate
* add documentation website

### Feat

* allow specifcation of full url for endpoint
* add config validation task
* add Runtime interface
* remove proxyd
* use the unified pkgs repo artifacts
* **osd:** Enable hitting multiple OSD endpoints

### Fix

* set packet and metal platform mode to metal
* always run networkd
* run only essential services in container mode
* add slub_debug=P to ISO kernel args
* use talos.config instead of talos.userdata
* use localhost for osd endpoint on masters
* check if cluster network config is nil

### Refactor

* improve validate flag names
* use go 1.13 error wrapping
* add helper func to create cert and key
* improve metal platform
* use control plane endpoint instead of master IPs


<a name="v0.3.0-alpha.1"></a>
## [v0.3.0-alpha.1](https://github.com/talos-systems/talos/compare/v0.2.0...v0.3.0-alpha.1) (2019-10-11)

### Chore

* prepare release v0.3.0-alpha.1
* update sonobuoy for conformance tests
* re-enable end to end tests
* enable 'wsl' linter and fix all the issues
* bump golangci-lint to 1.20
* Improve error messages if there is a network config overlap
* Add additional cert info to etcd peer cert.
* prepare release v0.2.0
* upgrade tools for go v1.13.1
* bump kernel to 5.2.18
* use the official Drone git plugin
* prepare release v0.3.0-alpha.0
* prepare release v0.2.0-rc.0
* add version label to installer image
* move gRPC API to public
* fix AWS image dependency

### Docs

* some docs improvements based on community feedback (try 2)
* Add machine config docs

### Feat

* add external IP discovery for azure
* add retry package
* output cluster network info for all node types
* default docker based cluster to 1 master
* add CNI, and pod and service CIDR to configurator
* use bootkube for cluster creation
* add configurator interface
* add etcd service
* Discover platform external addresses
* Add kubeadm flex on etcd if service is enabled
* add etcd service to config
* Add etcd ca generation to userdata.Generate
* discover control plane endpoints via Kubernetes
* Allow env override of hack/qemu image location
* allow Kubernetes version to be configured
* use kubeadm to distribute Kubernetes PKI
* write audit policy instead of using trustd
* add aescbcEncryptionSecret field to machine config
* return a struct for processes RPC
* default processes command to one shot
* upgrade Kubernetes to v1.16.0
* return a data structure in version RPC

### Fix

* retry endpoint discovery
* Make updating cert sans an append operation
* Use correct names for kubelet config
* generate admin client certificate with 10 year expiration
* always write the config to disk
* marshal v1alpha1 config in String() method
* update platform task to set hostname and cert SANs
* set --cluster-dns kubelet flag properly
* set kubelet-preferred-address-types to prioritize InternalIP
* catch panics in boot go routine
* set target if specified on command line
* update bootkube fork to fix pod-checkpointer
* ignore case in install platform check
* create etcd data directory
* generate CA certificates with 10 year expiration
* set extra kernel args for all platforms
* generate CA certificates with 1 year expiration
* add kerenel config required by Cilium
* **machined:** add nil checks to metal initializer

### Refactor

* decouple grpc client and userdata code
* rename RPCs


<a name="v0.2.0"></a>
## [v0.2.0](https://github.com/talos-systems/talos/compare/v0.3.0-alpha.0...v0.2.0) (2019-10-03)

### Chore

* prepare release v0.2.0
* prepare release v0.2.0-rc.0
* fix AWS image dependency

### Feat

* upgrade Kubernetes to v1.16.0

### Fix

* set default node identity cert to expire in 10 years
* generate CA certificates with 10 year expiration
* generate CA certificates with 1 year expiration
* add kerenel config required by Cilium
* **machined:** add nil checks to metal initializer


<a name="v0.3.0-alpha.0"></a>
## [v0.3.0-alpha.0](https://github.com/talos-systems/talos/compare/v0.2.0-rc.0...v0.3.0-alpha.0) (2019-09-23)

### Chore

* prepare release v0.3.0-alpha.0
* prepare release v0.2.0-rc.0
* add version label to installer image
* move gRPC API to public
* fix AWS image dependency

### Feat

* return a struct for processes RPC
* default processes command to one shot
* upgrade Kubernetes to v1.16.0
* return a data structure in version RPC

### Fix

* generate CA certificates with 10 year expiration
* set extra kernel args for all platforms
* generate CA certificates with 1 year expiration
* add kerenel config required by Cilium
* **machined:** add nil checks to metal initializer

### Refactor

* rename RPCs


<a name="v0.2.0-rc.0"></a>
## [v0.2.0-rc.0](https://github.com/talos-systems/talos/compare/v0.2.0-beta.0...v0.2.0-rc.0) (2019-09-23)

### Chore

* prepare release v0.2.0-rc.0
* fix AWS image dependency

### Feat

* upgrade Kubernetes to v1.16.0

### Fix

* generate CA certificates with 10 year expiration
* generate CA certificates with 1 year expiration
* add kerenel config required by Cilium
* **machined:** add nil checks to metal initializer


<a name="v0.2.0-beta.0"></a>
## [v0.2.0-beta.0](https://github.com/talos-systems/talos/compare/v0.2.0-alpha.7...v0.2.0-beta.0) (2019-09-18)

### Chore

* prepare release v0.2.0-beta.0
* upgrade Sonobuoy to v0.15.4
* remove dead code
* upgrade conformange image
* make ntpd depend on networkd
* update github.com/stretchr/testify library to 1.4.0
* move interface type assertion to unit-tests
* randomize containerd namespace in tests
* make TestRunRestartFailed test more reliable
* move from gofumpt to gofumports
* add fmt target
* upgrade golancgi-lint to 1.18.0
* format code with gofumpt
* lint protobuf definitions
* remove invalid TODO
* remove unneeded packages
* remove existing AMI
* remove packer from installer
* rename v1 node configs to v1alpha1
* Rename maintainers channel
* remove top output border
* remove local upgrade functionality
* update provider-components for capi v0.1.9
* Retry check for HA control plane
* align time command with output standards
* remove generated raw disk
* remove buildkit cache directory
* enable unit-tests-race
* make TestContainerdSuite/TestRunTwice more robust
* make health tests more robust
* disable go test result cache
* fix generate version flag and mark v0 as deprecated
* fix location of Go build cache mount for unit-tests-race
* Fix azure image upload
* Clean up e2e scripts
* change upgrade request "url" to "image"
* remove unused init token
* output top header in all caps
* **machined:** implement process reaper for PID 1 machined process
* **machined:** Increase pid_max to 262k
* **machined:** Clean up unnecessary ticker alloc

### Docs

* add machine configuration proposal

### Feat

* upgrade Kubernetes to v1.16.0-rc.2
* upgrade Kubernetes to v1.16.0-rc.1
* move node certificate to tmpfs
* set expiry of certificates to 24 hours
* Allow spec of canonical controlplane addr
* allow network interface to be ignored
* configure interfaces concurrently
* **machined:** filter actions stop/start/restart on per-service level

### Fix

* increase retries for DHCP
* conditionally set log path
* generate client admin cert with 1 year expiry
* use /var/log for default log path
* enable slub_debug=P
* move to per-platform console setup
* use unique variables for CLI flags
* Add retry/delay to probing device file
* leave etcd when upgrading control plane node
* use ntp client constructor
* translate machine.network to networking.os
* log system services to /run/system/log
* prepend custom options for kernel commandline
* remove basic integration teardown
* prevent EBUSY when unmounting system disk
* set default install image
* make --target persistent across all commands
* **init:** Enable containerd subreaper
* **machined:** limit max stderr output, use pkg/cmd consistently
* **machined:** Fix hostname value when retrieving from cloud providers
* **networkd:** Fix hostname retrieval
* **osctl:** use real userdata as defaults for install
* **osd:** Mount host directory for grpc sockets

### Refactor

* **ntpd:** Improvements to the robustness of ntp

### Test

* **ci:** Add aws for e2e and conformance targets


<a name="v0.2.0-alpha.7"></a>
## [v0.2.0-alpha.7](https://github.com/talos-systems/talos/compare/v0.2.0-alpha.6...v0.2.0-alpha.7) (2019-08-27)

### Chore

* prepare release v0.2.0-alpha.7
* Increase timers for healthchecks
* upgrade tools
* use kubeadm v1beta2 structs everywhere
* fix qemu-boot.sh
* add QEMU script
* disable CIS benchmarks
* enable floating IP creation in e2e tests
* Make losetup atomic during installation
* Fix reread error value on retry
* enforce one sentence per line in Markdown files
* add markdownlint
* Retry reread partition table if EBUSY
* Add log message for userdata backoff.
* move to smaller azure instance type
* Disable rate limited kmessage
* remove sonobuoy spinner
* apply manifests when init node is ready
* update tools image
* update go modules to use Kubernetes v1.16.0-alpha.3
* use go runner in sonobuoy
* add kernel parameters doc for bare-metal
* **ci:** Only push `latest` tags if branch is master.

### Docs

* Add Azure docs

### Feat

* run installs via container
* upgrade kubernetes to v1.16.0-beta.1
* perform upgrades via container
* generate and use v1 machine configs
* add ability to pass data on event bus
* Allow hostname to be specified in userdata
* rename DATA partition to EPHEMERAL
* add sequencer interface
* add overlay task
* use BLKPG ioctl for partition events
* allow specification of additional API SANs
* run dedicated instance of containerd for system services
* mount /sys/fs/bpf
* add standardized command runner
* Add gRPC server for ntp
* upgrade Linux to v5.2.8
* **networkd:** Add grpc endpoint
* **osd:** Add ntpd client
* **proxyd:** Add gRPC server

### Fix

* enclose target in quotes
* verify installation definition
* name the serde functions appropriately
* **gpt:** Fix partition naming to be >8 characters
* **machined:** Remove host mounts for specific CNI providers

### Refactor

* add more runtime modes
* **networkd:** Switch from rtnetlink to rtnl
* **networkd:** Replace networkd with a standalone app
* **proxyd:** Update multilisteners to use error chan.


<a name="v0.2.0-alpha.6"></a>
## [v0.2.0-alpha.6](https://github.com/talos-systems/talos/compare/v0.2.0-alpha.5...v0.2.0-alpha.6) (2019-08-13)

### Chore

* fix drone clone
* fix default pipeline
* fix release pipeline
* prepare release v0.2.0-alpha.6
* run unique E2E tests
* exclude promotion event
* add ability to promote to a release
* add image test step
* reenable AMI publishing
* refactor the Jsonnet file
* fix push step dependencies
* fix clone logic
* fix broken clone
* build drone YAML via jsonnet
* remove GitHub action workflow
* Fix up adhoc e2e tests
* add race-enabled test run
* remove machined from rootfs target
* add azure e2e testing
* re-add github actions
* delete github actions temporarily
* set docker server entrypoint to dockerd to avoid TLS generation
* enable CIS testing in conformance runs
* stabilize one more health test
* **ci:** Add e2e promotion pipeline

### Docs

* add project layout standards

### Feat

* remove the machine config on reset
* upgrade kubernetes to v1.16.0-alpha.3
* bump k8s version to v1.15.2

### Fix

* format IPv6 host entries properly
* store PartitionName when on NVMe disk
* stalls in local Docker cluster boot
* enable IPv6 forwarding
* enclose address in brackets gRPC client
* enclose server address is bracks if IPv6
* **initramfs:** Allow data partition to grow
* **machined:** Clean up installation process
* **proxyd:** print bootstrap backend dial errors
* **proxyd:** do not pre-bracket IPv6 backend addrs
* **proxyd:** wrap Dial addresses


<a name="v0.2.0-alpha.5"></a>
## [v0.2.0-alpha.5](https://github.com/talos-systems/talos/compare/v0.1.0...v0.2.0-alpha.5) (2019-08-05)

### Chore

* prepare release v0.2.0-alpha.5
* stabilize health test
* fix data race in goroutine runner
* add tests for event.Bus
* add GitHub action to enforce conform policies ([#803](https://github.com/talos-systems/talos/issues/803))
* run CI jobs on CI nodes
* update dockerfile/buildkit versions
* prepare release v0.1.0
* remove rootfs output param
* prepare release v0.2.0-alpha.4
* add AMI build
* remove hack/dev/ scripts & docker-compose
* implement first version of CRI runner
* fix build cache
* create raw image as sparse file
* fix GOCACHE dir location
* allow to run tests only for specified packages
* compress Azure image
* remove the raw disk after Azure build
* fix release
* fix image builds on tags
* prepare v0.2.0-alpha.3 release
* setup gce for e2e builds
* repair 'make all'
* run tests in the buildkit itself
* prepare release v0.1.0-rc.0
* publish Azure image on releases
* add step to drone for kernel
* prepare release v0.2.0-alpha.2
* move init to /sbin
* improve network setup logging
* extract CRI client as separate package
* make unit-tests use isolated instances of containerd
* prevent duplicate build of test container
* bump codecov project target to 33%
* remove last updated field from proposal template
* prepare release v0.2.0-alpha.1
* update toolchain version and output created config files
* prepare release v0.1.0-beta.1
* upgrade conform to v0.1.0-alpha.16
* upgrade conform to v0.1.0-alpha.15
* use 'fast' gitmeta ([#836](https://github.com/talos-systems/talos/issues/836))
* fix CHANGELOGs ([#834](https://github.com/talos-systems/talos/issues/834))
* create a CHANGELOG.md for each minor version ([#833](https://github.com/talos-systems/talos/issues/833))
* update stretchr/testify to master version ([#832](https://github.com/talos-systems/talos/issues/832))
* fix GCE image creation ([#830](https://github.com/talos-systems/talos/issues/830))
* revert [#816](https://github.com/talos-systems/talos/issues/816) ([#829](https://github.com/talos-systems/talos/issues/829))
* fix GCE image creation ([#816](https://github.com/talos-systems/talos/issues/816))
* upgrade conform to v0.1.0-alpha.14 ([#825](https://github.com/talos-systems/talos/issues/825))
* fix CHANGELOG ([#814](https://github.com/talos-systems/talos/issues/814))
* prepare release v0.1.0-beta.1 ([#811](https://github.com/talos-systems/talos/issues/811))
* publish gce images with releases ([#809](https://github.com/talos-systems/talos/issues/809))
* upgrade conform to v0.1.0-alpha.13 ([#808](https://github.com/talos-systems/talos/issues/808))
* use pull_request event for GitHub action ([#805](https://github.com/talos-systems/talos/issues/805))
* fix GitHub action ([#804](https://github.com/talos-systems/talos/issues/804))
* seed math.rand PRNG on startup in every service ([#801](https://github.com/talos-systems/talos/issues/801))
* **ci:** download golangci-lint only once ([#802](https://github.com/talos-systems/talos/issues/802))
* **init:** rearrange phase handling to push shutdown to main

### Docs

* minor spelling corrections.

### Feat

* add install flag for extra kernel args
* break up osctl cluster create and basic/e2e tests
* Use individual component steps for drone
* run rootfs from squashfs
* enable missing KSPP sysctls
* move df API to init
* attempt to connect to all trustd endpoints when downloading PKI
* set default mtu for gce platform
* allow mtu specification for network devices
* allow specification of mtu for cluster create
* add machined
* update kernel
* use new pkgs for initramfs and rootfs
* **init:** implement complete API for service lifecycle (start/stop)
* **init:** Add azure as a supported platform
* **initramfs:** Add kernel arg for default interface
* **osctl:** allow configurable number of masters to `cluster create`
* **osd:** implement container metrics for CRI inspector ([#824](https://github.com/talos-systems/talos/issues/824))
* **osd:** implement CRI inspector for containers ([#817](https://github.com/talos-systems/talos/issues/817))

### Fix

* return non-nil response in reset
* create overlay mounts after install
* probe specified install device ([#818](https://github.com/talos-systems/talos/issues/818))
* set mtu value regardless of interface state
* Run cleanup script earlier in rootfs build
* mount cgroups properly
* use existing logic to perform reset
* check proper value of parseip in dhcp
* make /etc/resolv.conf writable
* Only generate pki from trustd if not control plane
* Truncate hostname if necessary
* prefix file stat with rootfs prefix
* create symlinks to /etc/ssl/certs
* Fix integration of extra kernel args
* append probed block devices
* mount the owned partitions in cloud platforms
* move to crypto/rand for token gen ([#794](https://github.com/talos-systems/talos/issues/794))
* **init:** Dont log an error when context canceled
* **init:** Fix routes endpoint
* **init:** flip concurrency of tasks/services, fix small issues
* **init:** secret data at rest encryption key should be truly random ([#797](https://github.com/talos-systems/talos/issues/797))
* **trustd:** allow hostnames for trustd endpoints

### Refactor

* restructure the project layout
* improve installation reliability
* split machined into phases
* move setup logic into machined
* Userdata.download supports functional args ([#819](https://github.com/talos-systems/talos/issues/819))


<a name="v0.1.0"></a>
## [v0.1.0](https://github.com/talos-systems/talos/compare/v0.2.0-alpha.4...v0.1.0) (2019-07-30)

### Chore

* prepare release v0.1.0
* prepare release v0.1.0-rc.0
* prepare release v0.1.0-beta.1
* fix CHANGELOGs ([#834](https://github.com/talos-systems/talos/issues/834)) ([#835](https://github.com/talos-systems/talos/issues/835))
* fix GCE image creation ([#830](https://github.com/talos-systems/talos/issues/830)) ([#831](https://github.com/talos-systems/talos/issues/831))
* fix CHANGELOG ([#814](https://github.com/talos-systems/talos/issues/814)) ([#815](https://github.com/talos-systems/talos/issues/815))
* prepare release v0.1.0-beta.1 ([#811](https://github.com/talos-systems/talos/issues/811)) ([#813](https://github.com/talos-systems/talos/issues/813))
* cherry-pick conform changes ([#812](https://github.com/talos-systems/talos/issues/812))
* publish gce images with releases ([#809](https://github.com/talos-systems/talos/issues/809)) ([#810](https://github.com/talos-systems/talos/issues/810))

### Feat

* attempt to connect to all trustd endpoints when downloading PKI

### Fix

* mount cgroups properly
* Only generate pki from trustd if not control plane
* use existing logic to perform reset
* use existing logic to perform reset
* append probed block devices
* probe specified install device ([#818](https://github.com/talos-systems/talos/issues/818)) ([#820](https://github.com/talos-systems/talos/issues/820))
* move to crypto/rand for token gen ([#794](https://github.com/talos-systems/talos/issues/794))
* **init:** secret data at rest encryption key should be truly random ([#799](https://github.com/talos-systems/talos/issues/799))


<a name="v0.2.0-alpha.4"></a>
## [v0.2.0-alpha.4](https://github.com/talos-systems/talos/compare/v0.2.0-alpha.3...v0.2.0-alpha.4) (2019-07-30)

### Chore

* remove rootfs output param
* prepare release v0.2.0-alpha.4
* add AMI build
* remove hack/dev/ scripts & docker-compose
* implement first version of CRI runner
* fix build cache
* create raw image as sparse file
* fix GOCACHE dir location
* allow to run tests only for specified packages

### Feat

* run rootfs from squashfs
* enable missing KSPP sysctls
* move df API to init
* attempt to connect to all trustd endpoints when downloading PKI

### Fix

* Run cleanup script earlier in rootfs build
* mount cgroups properly
* check proper value of parseip in dhcp
* **trustd:** allow hostnames for trustd endpoints

### Refactor

* split machined into phases
* move setup logic into machined


<a name="v0.2.0-alpha.3"></a>
## [v0.2.0-alpha.3](https://github.com/talos-systems/talos/compare/v0.1.0-rc.0...v0.2.0-alpha.3) (2019-07-22)

### Chore

* compress Azure image
* remove the raw disk after Azure build
* fix release
* fix image builds on tags
* prepare v0.2.0-alpha.3 release
* setup gce for e2e builds
* repair 'make all'
* run tests in the buildkit itself
* prepare release v0.1.0-rc.0
* publish Azure image on releases
* add step to drone for kernel
* prepare release v0.2.0-alpha.2
* move init to /sbin
* improve network setup logging
* extract CRI client as separate package
* make unit-tests use isolated instances of containerd
* prevent duplicate build of test container
* bump codecov project target to 33%
* remove last updated field from proposal template
* prepare release v0.2.0-alpha.1
* update toolchain version and output created config files
* prepare release v0.1.0-beta.1
* upgrade conform to v0.1.0-alpha.16
* upgrade conform to v0.1.0-alpha.15
* use 'fast' gitmeta ([#836](https://github.com/talos-systems/talos/issues/836))
* fix CHANGELOGs ([#834](https://github.com/talos-systems/talos/issues/834))
* create a CHANGELOG.md for each minor version ([#833](https://github.com/talos-systems/talos/issues/833))
* update stretchr/testify to master version ([#832](https://github.com/talos-systems/talos/issues/832))
* fix GCE image creation ([#830](https://github.com/talos-systems/talos/issues/830))
* revert [#816](https://github.com/talos-systems/talos/issues/816) ([#829](https://github.com/talos-systems/talos/issues/829))
* fix GCE image creation ([#816](https://github.com/talos-systems/talos/issues/816))
* upgrade conform to v0.1.0-alpha.14 ([#825](https://github.com/talos-systems/talos/issues/825))
* fix CHANGELOG ([#814](https://github.com/talos-systems/talos/issues/814))
* prepare release v0.1.0-beta.1 ([#811](https://github.com/talos-systems/talos/issues/811))
* publish gce images with releases ([#809](https://github.com/talos-systems/talos/issues/809))
* upgrade conform to v0.1.0-alpha.13 ([#808](https://github.com/talos-systems/talos/issues/808))
* use pull_request event for GitHub action ([#805](https://github.com/talos-systems/talos/issues/805))
* fix GitHub action ([#804](https://github.com/talos-systems/talos/issues/804))
* add GitHub action to enforce conform policies ([#803](https://github.com/talos-systems/talos/issues/803))
* seed math.rand PRNG on startup in every service ([#801](https://github.com/talos-systems/talos/issues/801))
* **ci:** download golangci-lint only once ([#802](https://github.com/talos-systems/talos/issues/802))

### Docs

* minor spelling corrections.

### Feat

* set default mtu for gce platform
* allow mtu specification for network devices
* allow specification of mtu for cluster create
* add machined
* use new pkgs for initramfs and rootfs
* add install flag for extra kernel args
* update kernel
* Use individual component steps for drone
* **init:** Add azure as a supported platform
* **initramfs:** Add kernel arg for default interface
* **osd:** implement container metrics for CRI inspector ([#824](https://github.com/talos-systems/talos/issues/824))
* **osd:** implement CRI inspector for containers ([#817](https://github.com/talos-systems/talos/issues/817))

### Fix

* make /etc/resolv.conf writable
* Only generate pki from trustd if not control plane
* Truncate hostname if necessary
* prefix file stat with rootfs prefix
* create symlinks to /etc/ssl/certs
* Fix integration of extra kernel args
* probe specified install device ([#818](https://github.com/talos-systems/talos/issues/818))
* return non-nil response in reset
* use existing logic to perform reset
* append probed block devices
* move to crypto/rand for token gen ([#794](https://github.com/talos-systems/talos/issues/794))
* **init:** Dont log an error when context canceled
* **init:** Fix routes endpoint
* **init:** secret data at rest encryption key should be truly random ([#797](https://github.com/talos-systems/talos/issues/797))

### Refactor

* Userdata.download supports functional args ([#819](https://github.com/talos-systems/talos/issues/819))


<a name="v0.1.0-rc.0"></a>
## [v0.1.0-rc.0](https://github.com/talos-systems/talos/compare/v0.2.0-alpha.2...v0.1.0-rc.0) (2019-07-17)

### Chore

* prepare release v0.1.0-rc.0
* prepare release v0.1.0-beta.1
* fix CHANGELOGs ([#834](https://github.com/talos-systems/talos/issues/834)) ([#835](https://github.com/talos-systems/talos/issues/835))
* fix GCE image creation ([#830](https://github.com/talos-systems/talos/issues/830)) ([#831](https://github.com/talos-systems/talos/issues/831))
* fix CHANGELOG ([#814](https://github.com/talos-systems/talos/issues/814)) ([#815](https://github.com/talos-systems/talos/issues/815))
* prepare release v0.1.0-beta.1 ([#811](https://github.com/talos-systems/talos/issues/811)) ([#813](https://github.com/talos-systems/talos/issues/813))
* cherry-pick conform changes ([#812](https://github.com/talos-systems/talos/issues/812))
* publish gce images with releases ([#809](https://github.com/talos-systems/talos/issues/809)) ([#810](https://github.com/talos-systems/talos/issues/810))

### Fix

* use existing logic to perform reset
* use existing logic to perform reset
* append probed block devices
* probe specified install device ([#818](https://github.com/talos-systems/talos/issues/818)) ([#820](https://github.com/talos-systems/talos/issues/820))
* move to crypto/rand for token gen ([#794](https://github.com/talos-systems/talos/issues/794))
* **init:** secret data at rest encryption key should be truly random ([#799](https://github.com/talos-systems/talos/issues/799))


<a name="v0.2.0-alpha.2"></a>
## [v0.2.0-alpha.2](https://github.com/talos-systems/talos/compare/v0.2.0-alpha.1...v0.2.0-alpha.2) (2019-07-15)

### Chore

* add step to drone for kernel
* prepare release v0.2.0-alpha.2
* move init to /sbin
* improve network setup logging
* extract CRI client as separate package
* make unit-tests use isolated instances of containerd
* prevent duplicate build of test container
* bump codecov project target to 33%
* remove last updated field from proposal template

### Feat

* use new pkgs for initramfs and rootfs
* add install flag for extra kernel args
* update kernel
* Use individual component steps for drone

### Fix

* return non-nil response in reset
* **init:** Fix routes endpoint
* **init:** Dont log an error when context canceled


<a name="v0.2.0-alpha.1"></a>
## [v0.2.0-alpha.1](https://github.com/talos-systems/talos/compare/v0.1.0-beta.1...v0.2.0-alpha.1) (2019-07-05)

### Chore

* prepare release v0.2.0-alpha.1
* update toolchain version and output created config files
* prepare release v0.1.0-beta.1
* upgrade conform to v0.1.0-alpha.16
* upgrade conform to v0.1.0-alpha.15
* use 'fast' gitmeta ([#836](https://github.com/talos-systems/talos/issues/836))
* fix CHANGELOGs ([#834](https://github.com/talos-systems/talos/issues/834))
* create a CHANGELOG.md for each minor version ([#833](https://github.com/talos-systems/talos/issues/833))
* update stretchr/testify to master version ([#832](https://github.com/talos-systems/talos/issues/832))
* fix GCE image creation ([#830](https://github.com/talos-systems/talos/issues/830))
* revert [#816](https://github.com/talos-systems/talos/issues/816) ([#829](https://github.com/talos-systems/talos/issues/829))
* fix GCE image creation ([#816](https://github.com/talos-systems/talos/issues/816))
* upgrade conform to v0.1.0-alpha.14 ([#825](https://github.com/talos-systems/talos/issues/825))
* fix CHANGELOG ([#814](https://github.com/talos-systems/talos/issues/814))
* prepare release v0.1.0-beta.1 ([#811](https://github.com/talos-systems/talos/issues/811))
* publish gce images with releases ([#809](https://github.com/talos-systems/talos/issues/809))
* upgrade conform to v0.1.0-alpha.13 ([#808](https://github.com/talos-systems/talos/issues/808))
* use pull_request event for GitHub action ([#805](https://github.com/talos-systems/talos/issues/805))
* fix GitHub action ([#804](https://github.com/talos-systems/talos/issues/804))
* add GitHub action to enforce conform policies ([#803](https://github.com/talos-systems/talos/issues/803))
* seed math.rand PRNG on startup in every service ([#801](https://github.com/talos-systems/talos/issues/801))
* **ci:** download golangci-lint only once ([#802](https://github.com/talos-systems/talos/issues/802))

### Feat

* **initramfs:** Add kernel arg for default interface
* **osd:** implement container metrics for CRI inspector ([#824](https://github.com/talos-systems/talos/issues/824))
* **osd:** implement CRI inspector for containers ([#817](https://github.com/talos-systems/talos/issues/817))

### Fix

* append probed block devices
* use existing logic to perform reset
* probe specified install device ([#818](https://github.com/talos-systems/talos/issues/818))
* move to crypto/rand for token gen ([#794](https://github.com/talos-systems/talos/issues/794))
* **init:** secret data at rest encryption key should be truly random ([#797](https://github.com/talos-systems/talos/issues/797))

### Refactor

* Userdata.download supports functional args ([#819](https://github.com/talos-systems/talos/issues/819))


<a name="v0.1.0-beta.1"></a>
## [v0.1.0-beta.1](https://github.com/talos-systems/talos/compare/v0.1.0-beta.0...v0.1.0-beta.1) (2019-07-05)

### Chore

* prepare release v0.1.0-beta.1
* fix CHANGELOGs ([#834](https://github.com/talos-systems/talos/issues/834)) ([#835](https://github.com/talos-systems/talos/issues/835))
* fix GCE image creation ([#830](https://github.com/talos-systems/talos/issues/830)) ([#831](https://github.com/talos-systems/talos/issues/831))
* fix CHANGELOG ([#814](https://github.com/talos-systems/talos/issues/814)) ([#815](https://github.com/talos-systems/talos/issues/815))
* prepare release v0.1.0-beta.1 ([#811](https://github.com/talos-systems/talos/issues/811)) ([#813](https://github.com/talos-systems/talos/issues/813))
* cherry-pick conform changes ([#812](https://github.com/talos-systems/talos/issues/812))
* publish gce images with releases ([#809](https://github.com/talos-systems/talos/issues/809)) ([#810](https://github.com/talos-systems/talos/issues/810))

### Fix

* append probed block devices
* probe specified install device ([#818](https://github.com/talos-systems/talos/issues/818)) ([#820](https://github.com/talos-systems/talos/issues/820))
* move to crypto/rand for token gen ([#794](https://github.com/talos-systems/talos/issues/794))
* **init:** secret data at rest encryption key should be truly random ([#799](https://github.com/talos-systems/talos/issues/799))


<a name="v0.1.0-beta.0"></a>
## [v0.1.0-beta.0](https://github.com/talos-systems/talos/compare/v0.2.0-alpha.0...v0.1.0-beta.0) (2019-06-26)


<a name="v0.2.0-alpha.0"></a>
## [v0.2.0-alpha.0](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.28...v0.2.0-alpha.0) (2019-06-26)

### Chore

* prepare release v0.1.0-beta.0 ([#792](https://github.com/talos-systems/talos/issues/792))
* disable e2e ([#769](https://github.com/talos-systems/talos/issues/769))
* remove ready plugin from CoreDNS ([#764](https://github.com/talos-systems/talos/issues/764))
* fix drone make command for basic and E2E integration tests ([#763](https://github.com/talos-systems/talos/issues/763))
* update floating IPs for E2E test ([#762](https://github.com/talos-systems/talos/issues/762))
* add e2e test ([#736](https://github.com/talos-systems/talos/issues/736))
* bump k8s version in makefile ([#758](https://github.com/talos-systems/talos/issues/758))
* tidy modules and verify module tidyness on build ([#757](https://github.com/talos-systems/talos/issues/757))
* update toolchain images ([#754](https://github.com/talos-systems/talos/issues/754))
* don't run tests in parallel across packages ([#748](https://github.com/talos-systems/talos/issues/748))
* improve test stability for containerd tests ([#733](https://github.com/talos-systems/talos/issues/733))
* add google group to readme ([#730](https://github.com/talos-systems/talos/issues/730))
* download official gitmeta to BINDIR ([#717](https://github.com/talos-systems/talos/issues/717))
* address flaky tests instability ([#713](https://github.com/talos-systems/talos/issues/713))
* **ci:** modularize integration test ([#722](https://github.com/talos-systems/talos/issues/722))

### Docs

* target developers in the README and users in the docs ([#791](https://github.com/talos-systems/talos/issues/791))
* update getting started guide ([#787](https://github.com/talos-systems/talos/issues/787))
* add use cases section ([#786](https://github.com/talos-systems/talos/issues/786))
* fix the everytimezone.com link ([#778](https://github.com/talos-systems/talos/issues/778))
* update menu ([#775](https://github.com/talos-systems/talos/issues/775))
* improve description and layout ([#774](https://github.com/talos-systems/talos/issues/774))
* refresh getting started guide ([#773](https://github.com/talos-systems/talos/issues/773))
* rename Google Cloud to GCP ([#772](https://github.com/talos-systems/talos/issues/772))
* bring in missing changes from docs repo ([#771](https://github.com/talos-systems/talos/issues/771))
* move docs repo to talos repo ([#770](https://github.com/talos-systems/talos/issues/770))

### Feat

* add support for upgrading init nodes ([#761](https://github.com/talos-systems/talos/issues/761))
* enable debug in udevd service ([#783](https://github.com/talos-systems/talos/issues/783))
* use eudev for udevd ([#780](https://github.com/talos-systems/talos/issues/780))
* **ci:** enable nightly e2e tests ([#716](https://github.com/talos-systems/talos/issues/716))
* **init:** Add support for stopping individual services ([#706](https://github.com/talos-systems/talos/issues/706))
* **init:** Add service stop api ([#708](https://github.com/talos-systems/talos/issues/708))
* **init:** move 'ls' API to init from osd ([#755](https://github.com/talos-systems/talos/issues/755))
* **init:** Add support for kubeadm reset during upgrade ([#714](https://github.com/talos-systems/talos/issues/714))
* **init:** unify filesystem walkers for `ls`/`cp` APIs ([#779](https://github.com/talos-systems/talos/issues/779))
* **init:** Implement 'ls' command ([#721](https://github.com/talos-systems/talos/issues/721))
* **osctl:** implement 'cp' to copy files out of the Talos node ([#740](https://github.com/talos-systems/talos/issues/740))
* **osctl:** improve output of `stats` and `ps` commands ([#788](https://github.com/talos-systems/talos/issues/788))
* **osd:** extend Routes API ([#756](https://github.com/talos-systems/talos/issues/756))

### Fix

* we don't need no stinkin' localapiendpoint ([#741](https://github.com/talos-systems/talos/issues/741))
* top-level docs now appear properly with sidebar ([#785](https://github.com/talos-systems/talos/issues/785))
* Add gitmeta as dependency for push ([#718](https://github.com/talos-systems/talos/issues/718))
* containers test by locking image to specific tag ([#734](https://github.com/talos-systems/talos/issues/734))
* update hack/dev for new userdata location ([#777](https://github.com/talos-systems/talos/issues/777))
* run basic-integration on nightly cron ([#735](https://github.com/talos-systems/talos/issues/735))
* ensure index remains in bounds for ud gen ([#710](https://github.com/talos-systems/talos/issues/710))
* **init:** Add modules mountpoint for kube services ([#767](https://github.com/talos-systems/talos/issues/767))
* **init:** fix leaky ticker ([#784](https://github.com/talos-systems/talos/issues/784))
* **init:** use 127.0.0.1 IP in healthchecks to avoid resolver weirdness ([#715](https://github.com/talos-systems/talos/issues/715))
* **osctl:** display non-fatal errors from ps/stats in osctl ([#724](https://github.com/talos-systems/talos/issues/724))
* **osctl:** Revert "display non-fatal errors from ps/stats in osctl ([#724](https://github.com/talos-systems/talos/issues/724))" ([#727](https://github.com/talos-systems/talos/issues/727))
* **osctl:** avoid panic on empty 'talosconfig' ([#725](https://github.com/talos-systems/talos/issues/725))
* **osctl:** allow '-target' flag for `osctl restart` ([#732](https://github.com/talos-systems/talos/issues/732))
* **proxyd:** Fix backend deletion ([#729](https://github.com/talos-systems/talos/issues/729))
* **proxyd:** Use local apiserver endpoint ([#776](https://github.com/talos-systems/talos/issues/776))
* **proxyd:** remove self-hosted label in listwatch ([#782](https://github.com/talos-systems/talos/issues/782))
* **proxyd:** Add support for dropping broken backends ([#790](https://github.com/talos-systems/talos/issues/790))

### Perf

* **proxyd:** filter listwatch and remove backend on non-running pod ([#781](https://github.com/talos-systems/talos/issues/781))

### Refactor

* **osctl:** use UserHomeDir to detect user home directory ([#749](https://github.com/talos-systems/talos/issues/749))
* **osd:** implement container inspector for a single container ([#720](https://github.com/talos-systems/talos/issues/720))

### Test

* **proxyd:** Add basic suite of tests ([#789](https://github.com/talos-systems/talos/issues/789))


<a name="v0.1.0-alpha.28"></a>
## [v0.1.0-alpha.28](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.27...v0.1.0-alpha.28) (2019-06-03)

### Chore

* prepare release v0.1.0-alpha.28 ([#687](https://github.com/talos-systems/talos/issues/687))
* enable GOPROXY for go modules ([#703](https://github.com/talos-systems/talos/issues/703))
* improve the basic integration test ([#685](https://github.com/talos-systems/talos/issues/685))

### Docs

* change meeting times to 24 hour format ([#675](https://github.com/talos-systems/talos/issues/675))
* add Zoom meeting schedule to README ([#674](https://github.com/talos-systems/talos/issues/674))

### Feat

* leave etcd before upgrading ([#702](https://github.com/talos-systems/talos/issues/702))
* upgrade Kubernetes to v1.15.0-beta.1 ([#696](https://github.com/talos-systems/talos/issues/696))
* **init:** Add support for control plane join config ([#700](https://github.com/talos-systems/talos/issues/700))
* **init:** update 'waiting' state descritpion when conditions change ([#698](https://github.com/talos-systems/talos/issues/698))
* **init:** Prioritize usage of local userdata ([#694](https://github.com/talos-systems/talos/issues/694))
* **init:** expose networkd as goroutine-based server ([#682](https://github.com/talos-systems/talos/issues/682))
* **init:** implement service dependencies, correct start and shutdown ([#680](https://github.com/talos-systems/talos/issues/680))
* **init:** Add initToken parameter to userdata ([#664](https://github.com/talos-systems/talos/issues/664))
* **osctl:** handle ^C by aborting context ([#693](https://github.com/talos-systems/talos/issues/693))

### Fix

* don't set BUILDKIT_CACHE to empty string in Makefile ([#705](https://github.com/talos-systems/talos/issues/705))
* ensure shebang at top of userdata ([#695](https://github.com/talos-systems/talos/issues/695))
* Address lint warning for unknown linter ([#676](https://github.com/talos-systems/talos/issues/676))
* **init:** consider 'finished' services to be 'up' ([#699](https://github.com/talos-systems/talos/issues/699))
* **init:** move directory creation to kubeadm pre-func ([#688](https://github.com/talos-systems/talos/issues/688))
* **osctl:** Generate correct config with master IPs ([#681](https://github.com/talos-systems/talos/issues/681))
* **osctl:** don't print message on first ^C ([#704](https://github.com/talos-systems/talos/issues/704))
* **osd:** consistent container ids in stats, ps and reset ([#707](https://github.com/talos-systems/talos/issues/707))
* **osd:** Sanitize request.id for log streams ([#673](https://github.com/talos-systems/talos/issues/673))

### Refactor

* change conditions to be interface, add descriptions ([#677](https://github.com/talos-systems/talos/issues/677))
* fix stream chunker & provide some tests ([#672](https://github.com/talos-systems/talos/issues/672))
* **init:** use 'switch' instead of long condition ([#701](https://github.com/talos-systems/talos/issues/701))
* **init:** Allow kubeadm init on controlplane ([#658](https://github.com/talos-systems/talos/issues/658))
* **osctl:** move cli code out of 'client' package ([#692](https://github.com/talos-systems/talos/issues/692))
* **osctl:** DRY up osctl sources by using common client setup ([#686](https://github.com/talos-systems/talos/issues/686))


<a name="v0.1.0-alpha.27"></a>
## [v0.1.0-alpha.27](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.26...v0.1.0-alpha.27) (2019-05-20)

### Chore

* prepare release v0.1.0-alpha.27 ([#671](https://github.com/talos-systems/talos/issues/671))

### Docs

* fix typo in README.md ([#655](https://github.com/talos-systems/talos/issues/655))

### Feat

* add plural alias of service command ([#670](https://github.com/talos-systems/talos/issues/670))
* use osctl in installer ([#654](https://github.com/talos-systems/talos/issues/654))
* add bootstrap token package ([#657](https://github.com/talos-systems/talos/issues/657))
* use github.com/mdlayher/kobject ([#653](https://github.com/talos-systems/talos/issues/653))
* **init:** implement healthchecks for the services ([#667](https://github.com/talos-systems/talos/issues/667))
* **init:** implement services list API and osctl service CLI ([#662](https://github.com/talos-systems/talos/issues/662))
* **init:** implement health checks for services ([#656](https://github.com/talos-systems/talos/issues/656))

### Fix

* add libressl to rootfs ([#659](https://github.com/talos-systems/talos/issues/659))
* **init:** fix containerd healthcheck leaking memory in init/containerd ([#661](https://github.com/talos-systems/talos/issues/661))
* **init:** mount /sys into kubelet container ([#660](https://github.com/talos-systems/talos/issues/660))
* **osctl:** output config without localAPIEndpoint ([#665](https://github.com/talos-systems/talos/issues/665))
* **osd:** Read talos service logs from file ([#663](https://github.com/talos-systems/talos/issues/663))

### Refactor

* fix filechunker not exiting on context cancel ([#668](https://github.com/talos-systems/talos/issues/668))


<a name="v0.1.0-alpha.26"></a>
## [v0.1.0-alpha.26](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.25...v0.1.0-alpha.26) (2019-05-13)

### Chore

* prepare release v0.1.0-alpha.26 ([#652](https://github.com/talos-systems/talos/issues/652))
* workaround flaky tests ([#651](https://github.com/talos-systems/talos/issues/651))
* remove AMI publish step ([#650](https://github.com/talos-systems/talos/issues/650))
* fix creation of syslinux config file ([#639](https://github.com/talos-systems/talos/issues/639))
* set PACKER_LOG=1 in AMI build ([#637](https://github.com/talos-systems/talos/issues/637))
* add env vars required for AMI publishing ([#635](https://github.com/talos-systems/talos/issues/635))
* publish AMIs on tags ([#633](https://github.com/talos-systems/talos/issues/633))
* move osinstall to cmd ([#620](https://github.com/talos-systems/talos/issues/620))

### Feat

* add helper package for cordon and drain ([#626](https://github.com/talos-systems/talos/issues/626))
* add support for UEFI ([#642](https://github.com/talos-systems/talos/issues/642))
* improve package for /proc/cmdline parsing and management ([#645](https://github.com/talos-systems/talos/issues/645))
* add ability to create multiple entries in extlinux.conf ([#636](https://github.com/talos-systems/talos/issues/636))
* update partition layout to accomodate upgrades ([#621](https://github.com/talos-systems/talos/issues/621))
* remove EC2 verification step ([#631](https://github.com/talos-systems/talos/issues/631))
* upgrade Linux to v4.19.40 ([#630](https://github.com/talos-systems/talos/issues/630))
* update toolchain ([#628](https://github.com/talos-systems/talos/issues/628))
* Add additional kubernetes certs ([#619](https://github.com/talos-systems/talos/issues/619))
* **init:** Add upgrade endpoint ([#623](https://github.com/talos-systems/talos/issues/623))
* **init:** core health check package ([#632](https://github.com/talos-systems/talos/issues/632))
* **osctl:** add flag for number of workers to create ([#625](https://github.com/talos-systems/talos/issues/625))

### Fix

* allow no trustd endpoints to be specified ([#634](https://github.com/talos-systems/talos/issues/634))
* use the correct param in root label check ([#622](https://github.com/talos-systems/talos/issues/622))
* Update filesystem check to open device as a device ([#641](https://github.com/talos-systems/talos/issues/641))
* **init:** unlink unix bind address ([#643](https://github.com/talos-systems/talos/issues/643))
* **init:** don't close ACPI listen handle too early ([#647](https://github.com/talos-systems/talos/issues/647))
* **osctl:** Fix panic on osctl df if error is returned ([#646](https://github.com/talos-systems/talos/issues/646))
* **osctl:** Fix formatting of command/args to be useful ([#638](https://github.com/talos-systems/talos/issues/638))
* **osctl:** output talosconfig on generate ([#627](https://github.com/talos-systems/talos/issues/627))
* **osd:** Use correct context in stats endpoint ([#644](https://github.com/talos-systems/talos/issues/644))

### Refactor

* use os.Remove instead of unix.Unlink ([#648](https://github.com/talos-systems/talos/issues/648))
* move osinstall into osctl ([#629](https://github.com/talos-systems/talos/issues/629))
* **initramfs:** Compose Install better ([#624](https://github.com/talos-systems/talos/issues/624))

### Test

* add another test case for setting kernel params ([#649](https://github.com/talos-systems/talos/issues/649))


<a name="v0.1.0-alpha.25"></a>
## [v0.1.0-alpha.25](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.24...v0.1.0-alpha.25) (2019-05-05)

### Chore

* prepare release v0.1.0-alpha.25 ([#615](https://github.com/talos-systems/talos/issues/615))
* build iso image ([#616](https://github.com/talos-systems/talos/issues/616))
* Fix kubeadm warnings ([#612](https://github.com/talos-systems/talos/issues/612))
* update codecov project threshold to 17% ([#609](https://github.com/talos-systems/talos/issues/609))
* fix install command in packer template ([#603](https://github.com/talos-systems/talos/issues/603))
* add make target for building AMIs ([#602](https://github.com/talos-systems/talos/issues/602))
* update example outputs in README ([#600](https://github.com/talos-systems/talos/issues/600))

### Feat

* add support for ISO based installations ([#606](https://github.com/talos-systems/talos/issues/606))
* Add calico manifests for local dev setup ([#608](https://github.com/talos-systems/talos/issues/608))
* Validate userdata ([#593](https://github.com/talos-systems/talos/issues/593))
* **init:** run udevd as a container ([#601](https://github.com/talos-systems/talos/issues/601))
* **osctl:** add config generate command

### Fix

* add support for trustd username and password auth back in ([#604](https://github.com/talos-systems/talos/issues/604))
* **init:** start udevd with parent cgroup devices ([#605](https://github.com/talos-systems/talos/issues/605))
* **osctl:** ensure image is present ([#599](https://github.com/talos-systems/talos/issues/599))


<a name="v0.1.0-alpha.24"></a>
## [v0.1.0-alpha.24](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.23...v0.1.0-alpha.24) (2019-04-29)

### Chore

* prepare release v0.1.0-alpha.24 ([#598](https://github.com/talos-systems/talos/issues/598))
* Make buildkit cache OS dependent ([#595](https://github.com/talos-systems/talos/issues/595))
* add proposals template ([#590](https://github.com/talos-systems/talos/issues/590))
* add slack notification to drone ([#589](https://github.com/talos-systems/talos/issues/589))
* disable codecov patch status ([#588](https://github.com/talos-systems/talos/issues/588))
* add codecov configuration file ([#587](https://github.com/talos-systems/talos/issues/587))
* **ci:** Update buildkit v0.5 ([#594](https://github.com/talos-systems/talos/issues/594))
* **ci:** apply manifests and wait for healthy nodes ([#580](https://github.com/talos-systems/talos/issues/580))

### Feat

* add package for generating userdata ([#574](https://github.com/talos-systems/talos/issues/574))
* add shutdown command ([#577](https://github.com/talos-systems/talos/issues/577))
* add ability to generate userdata secrets ([#581](https://github.com/talos-systems/talos/issues/581))
* **init:** load only the images required by the node type ([#582](https://github.com/talos-systems/talos/issues/582))
* **init:** Add support for hostname kernel parameter ([#591](https://github.com/talos-systems/talos/issues/591))
* **init:** implement init gRPC API, forward reboot to init ([#579](https://github.com/talos-systems/talos/issues/579))
* **init:** enforce KSPP kernel parameters ([#585](https://github.com/talos-systems/talos/issues/585))
* **init:** implement graceful shutdown of 'init' ([#562](https://github.com/talos-systems/talos/issues/562))
* **osctl:** add ability to create docker based clusters ([#584](https://github.com/talos-systems/talos/issues/584))
* **osctl:** expose osd and api server ports on master-1 ([#592](https://github.com/talos-systems/talos/issues/592))
* **osctl:** add df command ([#569](https://github.com/talos-systems/talos/issues/569))
* **trustd:** use a token instead of username and password ([#586](https://github.com/talos-systems/talos/issues/586))

### Fix

* Explicitly set upstream/forward servers for coredns in dev setup ([#578](https://github.com/talos-systems/talos/issues/578))
* **osctl:** fix issue with downloading image ([#597](https://github.com/talos-systems/talos/issues/597))
* **osd:** read log files only on write events ([#583](https://github.com/talos-systems/talos/issues/583))

### Refactor

* **init:** add helper for getting specific kernel parameters ([#596](https://github.com/talos-systems/talos/issues/596))


<a name="v0.1.0-alpha.23"></a>
## [v0.1.0-alpha.23](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.22...v0.1.0-alpha.23) (2019-04-24)

### Chore

* prepare release v0.1.0-alpha.23 ([#565](https://github.com/talos-systems/talos/issues/565))
* Update kernel image ([#564](https://github.com/talos-systems/talos/issues/564))
* use the rootfs-base and initramfs-base images for builds ([#558](https://github.com/talos-systems/talos/issues/558))
* add slack invite badge ([#555](https://github.com/talos-systems/talos/issues/555))
* **ci:** Update buildkit to 0.4 ([#538](https://github.com/talos-systems/talos/issues/538))

### Docs

* update README.md with drone build status ([#552](https://github.com/talos-systems/talos/issues/552))

### Feat

* detect if an install has already occurred ([#549](https://github.com/talos-systems/talos/issues/549))
* **osctl:** Add osctl top ([#560](https://github.com/talos-systems/talos/issues/560))

### Fix

* **osd:** Fix k8s.io namespace logs ([#557](https://github.com/talos-systems/talos/issues/557))
* **osd:** Fix osctl ps output ([#554](https://github.com/talos-systems/talos/issues/554))

### Refactor

* extract 'restart' piece of the runners into wrapper runner ([#559](https://github.com/talos-systems/talos/issues/559))
* add stub unit-tests to non-trivial Go packages ([#556](https://github.com/talos-systems/talos/issues/556))
* add unit-test for containerd image import ([#553](https://github.com/talos-systems/talos/issues/553))
* containerd runner refactoring and unit-tests ([#551](https://github.com/talos-systems/talos/issues/551))


<a name="v0.1.0-alpha.22"></a>
## [v0.1.0-alpha.22](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.21...v0.1.0-alpha.22) (2019-04-16)

### Chore

* ignore checksum files create in release ([#550](https://github.com/talos-systems/talos/issues/550))
* remove modules from build output ([#548](https://github.com/talos-systems/talos/issues/548))
* remove release target in favor of build target ([#547](https://github.com/talos-systems/talos/issues/547))
* optimize the build for pull requests and tags ([#546](https://github.com/talos-systems/talos/issues/546))
* use gitmeta for image tag ([#545](https://github.com/talos-systems/talos/issues/545))
* improve drone parallel steps ([#544](https://github.com/talos-systems/talos/issues/544))
* fetch git tags ([#543](https://github.com/talos-systems/talos/issues/543))
* add BUILDKIT_HOST env var to release step ([#542](https://github.com/talos-systems/talos/issues/542))
* prepare release v0.1.0-alpha.22 ([#541](https://github.com/talos-systems/talos/issues/541))
* add github-release plugin ([#540](https://github.com/talos-systems/talos/issues/540))
* add dev-test make target to quickly re-run unit-tests ([#539](https://github.com/talos-systems/talos/issues/539))
* remove travis integration ([#535](https://github.com/talos-systems/talos/issues/535))
* push images for all branches ([#534](https://github.com/talos-systems/talos/issues/534))
* move codecov to drone build ([#533](https://github.com/talos-systems/talos/issues/533))
* split 'base' target, run tests in docker container ([#528](https://github.com/talos-systems/talos/issues/528))
* don't crate /lib/modules in  symlink.sh ([#529](https://github.com/talos-systems/talos/issues/529))
* create /lib/modules ([#527](https://github.com/talos-systems/talos/issues/527))
* keep buildkitd cache as local volume ([#522](https://github.com/talos-systems/talos/issues/522))
* fix push step ([#526](https://github.com/talos-systems/talos/issues/526))
* push images for master branch ([#525](https://github.com/talos-systems/talos/issues/525))
* add drone build ([#523](https://github.com/talos-systems/talos/issues/523))
* enforce go.mod completeness and better buildkit cache ([#520](https://github.com/talos-systems/talos/issues/520))
* clean up outer variable used in inner func ([#519](https://github.com/talos-systems/talos/issues/519))
* refactor container image import code to avoid panics ([#518](https://github.com/talos-systems/talos/issues/518))
* provide /etc/resolv.conf to kubelet & kubeadm ([#493](https://github.com/talos-systems/talos/issues/493))
* rework process runner, add tests and stop method ([#506](https://github.com/talos-systems/talos/issues/506))
* add goreportcard badge ([#516](https://github.com/talos-systems/talos/issues/516))
* upgrade golangci-lint to v1.16.0 ([#515](https://github.com/talos-systems/talos/issues/515))
* expose crypto package ([#512](https://github.com/talos-systems/talos/issues/512))
* add codecov integration ([#510](https://github.com/talos-systems/talos/issues/510))
* export coverage info from unit-tests ([#505](https://github.com/talos-systems/talos/issues/505))

### Docs

* refer to talos as an operating system ([#517](https://github.com/talos-systems/talos/issues/517))

### Feat

* remove blockd ([#536](https://github.com/talos-systems/talos/issues/536))
* log the xfs_growfs of the data partition ([#537](https://github.com/talos-systems/talos/issues/537))
* upgrade containerd to v1.2.6 ([#532](https://github.com/talos-systems/talos/issues/532))
* upgrade Linux to v4.19.34 ([#531](https://github.com/talos-systems/talos/issues/531))
* upgrade Kubernetes to v1.14.1 ([#530](https://github.com/talos-systems/talos/issues/530))
* add support for extra disk management ([#524](https://github.com/talos-systems/talos/issues/524))
* **initramfs:** Add support for specifying static routes ([#513](https://github.com/talos-systems/talos/issues/513))

### Fix

* remove redundant netlink connection, use netlink.IsNotExist in init ([#511](https://github.com/talos-systems/talos/issues/511))

### Refactor

* **initramfs:** clean up network code ([#507](https://github.com/talos-systems/talos/issues/507))


<a name="v0.1.0-alpha.21"></a>
## [v0.1.0-alpha.21](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.20...v0.1.0-alpha.21) (2019-04-08)

### Chore

* prepare release v0.1.0-alpha.21 ([#504](https://github.com/talos-systems/talos/issues/504))
* add basic integration test ([#502](https://github.com/talos-systems/talos/issues/502))
* add /var/log as a volume to docker platform ([#503](https://github.com/talos-systems/talos/issues/503))
* add container for development ([#501](https://github.com/talos-systems/talos/issues/501))
* fixups for ProcessLog ([#494](https://github.com/talos-systems/talos/issues/494))
* export vmlinux kernel ([#500](https://github.com/talos-systems/talos/issues/500))
* run lint and test first ([#496](https://github.com/talos-systems/talos/issues/496))
* refactor and dry up process runner ([#495](https://github.com/talos-systems/talos/issues/495))
* take osctl/kubectl out of docker-compose ([#492](https://github.com/talos-systems/talos/issues/492))
* fixes for talos in docker-compose environment ([#488](https://github.com/talos-systems/talos/issues/488))
* add release target to Makefile ([#490](https://github.com/talos-systems/talos/issues/490))
* make provided certificateKey 32 bytes ([#489](https://github.com/talos-systems/talos/issues/489))
* switch back docker image org name to 'autonomy' ([#487](https://github.com/talos-systems/talos/issues/487))
* fix Twitter badge ([#486](https://github.com/talos-systems/talos/issues/486))
* remove static images directory ([#485](https://github.com/talos-systems/talos/issues/485))
* move docs to a dedicated repo ([#484](https://github.com/talos-systems/talos/issues/484))
* remove 'Autonomy' from os-release ([#483](https://github.com/talos-systems/talos/issues/483))
* move website to netlify ([#482](https://github.com/talos-systems/talos/issues/482))
* upgrade DHCP package ([#481](https://github.com/talos-systems/talos/issues/481))
* update org to new name ([#480](https://github.com/talos-systems/talos/issues/480))
* expose userdata and osctl client packages ([#471](https://github.com/talos-systems/talos/issues/471))

### Feat

* add network configuration support ([#476](https://github.com/talos-systems/talos/issues/476))

### Fix

* create GCE disk as disk.raw ([#498](https://github.com/talos-systems/talos/issues/498))
* check link state before bringing it up ([#497](https://github.com/talos-systems/talos/issues/497))
* remove static resolv.conf ([#491](https://github.com/talos-systems/talos/issues/491))
* **osctl:** add missing flags ([#479](https://github.com/talos-systems/talos/issues/479))

### Refactor

* improve artifact names ([#499](https://github.com/talos-systems/talos/issues/499))


<a name="v0.1.0-alpha.20"></a>
## [v0.1.0-alpha.20](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.19...v0.1.0-alpha.20) (2019-04-02)

### Chore

* prepare release v0.1.0-alpha.20 ([#478](https://github.com/talos-systems/talos/issues/478))
* report errors in osctl cli in a consistent way ([#477](https://github.com/talos-systems/talos/issues/477))
* DRY userdata Kubeadm struct marshal/unmarshal ([#475](https://github.com/talos-systems/talos/issues/475))
* split ignorePreflightErrors as settings on its own ([#474](https://github.com/talos-systems/talos/issues/474))
* use protobuf compiler from the toolchain image ([#468](https://github.com/talos-systems/talos/issues/468))
* improve error reporting in osctl cli ([#467](https://github.com/talos-systems/talos/issues/467))
* **tools:** use Go compiler from toolchain image ([#460](https://github.com/talos-systems/talos/issues/460))

### Feat

* add packet support ([#473](https://github.com/talos-systems/talos/issues/473))
* upgrade runc to v1.0.0-rc.7 ([#469](https://github.com/talos-systems/talos/issues/469))
* upgrade Kubernetes to v1.14.0 ([#466](https://github.com/talos-systems/talos/issues/466))
* upgrade Linux to v4.19.31 ([#464](https://github.com/talos-systems/talos/issues/464))
* upgrade containerd to v1.2.5 ([#463](https://github.com/talos-systems/talos/issues/463))
* add power off functionality ([#462](https://github.com/talos-systems/talos/issues/462))
* add basic ntp implementation ([#459](https://github.com/talos-systems/talos/issues/459))
* install bootloader to block device ([#455](https://github.com/talos-systems/talos/issues/455))
* remove DenyEscalatingExec admission plugin ([#457](https://github.com/talos-systems/talos/issues/457))
* dd bootloader components ([#438](https://github.com/talos-systems/talos/issues/438))
* **initramfs:** add support for refreshing dhcp lease ([#454](https://github.com/talos-systems/talos/issues/454))

### Fix

* revert runc to v1.0.0-rc.6 ([#470](https://github.com/talos-systems/talos/issues/470))

### Refactor

* improve installer code ([#472](https://github.com/talos-systems/talos/issues/472))
* **initramfs:** verify shared mounts with kubelet ([#461](https://github.com/talos-systems/talos/issues/461))


<a name="v0.1.0-alpha.19"></a>
## [v0.1.0-alpha.19](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.18...v0.1.0-alpha.19) (2019-03-11)

### Chore

* prepare release v0.1.0-alpha.19 ([#448](https://github.com/talos-systems/talos/issues/448))
* upgrade Golang to v1.12.0 ([#452](https://github.com/talos-systems/talos/issues/452))
* upgrade conform ([#440](https://github.com/talos-systems/talos/issues/440))
* update go modules ([#429](https://github.com/talos-systems/talos/issues/429))
* create images that consider the size of /var ([#441](https://github.com/talos-systems/talos/issues/441))
* fix git commit hook ([#431](https://github.com/talos-systems/talos/issues/431))
* improve Makefile for newcomers ([#419](https://github.com/talos-systems/talos/issues/419))
* fix installer image name ([#394](https://github.com/talos-systems/talos/issues/394))
* fix Travis double builds ([#380](https://github.com/talos-systems/talos/issues/380))
* upgrade conform to v0.1.0-alpha.10 ([#379](https://github.com/talos-systems/talos/issues/379))
* upgrade golangci-lint to v1.14.0 ([#366](https://github.com/talos-systems/talos/issues/366))

### Docs

* remove ip=dhcp flag from documentation ([#428](https://github.com/talos-systems/talos/issues/428))
* improve contributing documentation ([#418](https://github.com/talos-systems/talos/issues/418))
* properly wrap layouts in html/body tags ([#411](https://github.com/talos-systems/talos/issues/411))
* add Twitter badge to README ([#405](https://github.com/talos-systems/talos/issues/405))
* add contact info to README ([#392](https://github.com/talos-systems/talos/issues/392))
* add comparison to similar distributions ([#352](https://github.com/talos-systems/talos/issues/352))
* fix Google Cloud example ([#391](https://github.com/talos-systems/talos/issues/391))
* fix badge for MPL license ([#371](https://github.com/talos-systems/talos/issues/371))
* fix typos in README.md and CONTRIBUTING.md ([#370](https://github.com/talos-systems/talos/issues/370))
* update master configuration documentation ([#359](https://github.com/talos-systems/talos/issues/359))

### Feat

* log to stdout when in container mode ([#450](https://github.com/talos-systems/talos/issues/450))
* upgrade containerd to v1.2.4 ([#395](https://github.com/talos-systems/talos/issues/395))
* add container based deploy support to init ([#447](https://github.com/talos-systems/talos/issues/447))
* add `docker-os` make target, Kubeadm.ExtraArgs, and a dev Makefile ([#446](https://github.com/talos-systems/talos/issues/446))
* add dosfstools to initramfs and rootfs ([#444](https://github.com/talos-systems/talos/issues/444))
* add arg to target nodes per command ([#435](https://github.com/talos-systems/talos/issues/435))
* update gcc to 8.3.0, drop gcompat ([#433](https://github.com/talos-systems/talos/issues/433))
* add DHCP client ([#427](https://github.com/talos-systems/talos/issues/427))
* allow user specified IP addresses in SANs ([#425](https://github.com/talos-systems/talos/issues/425))
* create certificates with all non-loopback addresses ([#424](https://github.com/talos-systems/talos/issues/424))
* add TALOSCONFIG env var ([#422](https://github.com/talos-systems/talos/issues/422))
* add automated PKI for joining nodes ([#406](https://github.com/talos-systems/talos/issues/406))
* add hostname to node certificate SAN ([#415](https://github.com/talos-systems/talos/issues/415))
* add config flag to osctl ([#413](https://github.com/talos-systems/talos/issues/413))
* add osinstall cli utility ([#368](https://github.com/talos-systems/talos/issues/368))
* add route printing to osctl ([#404](https://github.com/talos-systems/talos/issues/404))
* upgrade linux to v4.19.23 ([#402](https://github.com/talos-systems/talos/issues/402))
* upgrade musl to 1.1.21 ([#401](https://github.com/talos-systems/talos/issues/401))
* change AWS instance type to t2.micro ([#399](https://github.com/talos-systems/talos/issues/399))
* add gcloud integration ([#385](https://github.com/talos-systems/talos/issues/385))
* **hack:** add osctl/kubelet dev tooling and document usage ([#449](https://github.com/talos-systems/talos/issues/449))

### Fix

* mount /dev/shm as tmpfs ([#445](https://github.com/talos-systems/talos/issues/445))
* add missing mounts and remove memory limits ([#442](https://github.com/talos-systems/talos/issues/442))
* add iptables to rootfs ([#378](https://github.com/talos-systems/talos/issues/378))
* join masters in serial ([#437](https://github.com/talos-systems/talos/issues/437))
* assign to existing target variable ([#436](https://github.com/talos-systems/talos/issues/436))
* fallback on IP address when DHCP reply has no hostname ([#432](https://github.com/talos-systems/talos/issues/432))
* distribute PKI from initial master to joining masters ([#426](https://github.com/talos-systems/talos/issues/426))
* write config changes to specified config file ([#416](https://github.com/talos-systems/talos/issues/416))
* delay `gitmeta` until needed in Makefile ([#407](https://github.com/talos-systems/talos/issues/407))
* output userdata fails, ignore numcpu for kubeadm ([#398](https://github.com/talos-systems/talos/issues/398))
* ensure DNS works in early boot ([#382](https://github.com/talos-systems/talos/issues/382))
* add initialization for userdata download ([#367](https://github.com/talos-systems/talos/issues/367))
* **initramfs:** fix case where we download a non archive file ([#421](https://github.com/talos-systems/talos/issues/421))

### Refactor

* Make userdata.Open userdata.Download consistent return types ([#363](https://github.com/talos-systems/talos/issues/363))
* **init:** make baremetal consume install package ([#414](https://github.com/talos-systems/talos/issues/414))

### Test

* add kernel pkg tests, improve parsing ([#430](https://github.com/talos-systems/talos/issues/430))


<a name="v0.1.0-alpha.18"></a>
## [v0.1.0-alpha.18](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.17...v0.1.0-alpha.18) (2019-02-15)

### Chore

* prepare v0.1.0-alpha.18 release ([#346](https://github.com/talos-systems/talos/issues/346))
* remove GPG requirement ([#341](https://github.com/talos-systems/talos/issues/341))

### Docs

* add kubeadm mention in README ([#344](https://github.com/talos-systems/talos/issues/344))

### Fix

* add libblkid to the rootfs ([#345](https://github.com/talos-systems/talos/issues/345))
* Minor adjustments to makefile ([#340](https://github.com/talos-systems/talos/issues/340))


<a name="v0.1.0-alpha.17"></a>
## [v0.1.0-alpha.17](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.16...v0.1.0-alpha.17) (2019-02-14)

### Chore

* prepare v0.1.0-alpha.17 release ([#339](https://github.com/talos-systems/talos/issues/339))
* add CONTRIBUTING.md ([#337](https://github.com/talos-systems/talos/issues/337))
* add build toolchain to makefile ([#338](https://github.com/talos-systems/talos/issues/338))
* update slack room ([#332](https://github.com/talos-systems/talos/issues/332))
* add slack notification to travis ([#330](https://github.com/talos-systems/talos/issues/330))

### Docs

* improve the README ([#333](https://github.com/talos-systems/talos/issues/333))

### Feat

* upgrade Kubernetes to v1.13.3 ([#335](https://github.com/talos-systems/talos/issues/335))
* disable session tickets ([#334](https://github.com/talos-systems/talos/issues/334))


<a name="v0.1.0-alpha.16"></a>
## [v0.1.0-alpha.16](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.15...v0.1.0-alpha.16) (2019-01-24)

### Chore

* prepare v0.1.0-alpha.16 release ([#331](https://github.com/talos-systems/talos/issues/331))
* output a raw image ([#306](https://github.com/talos-systems/talos/issues/306))
* update README with a link to the documentation ([#327](https://github.com/talos-systems/talos/issues/327))
* update README badges ([#326](https://github.com/talos-systems/talos/issues/326))
* add travis config ([#321](https://github.com/talos-systems/talos/issues/321))
* pin AWS AMI ([#325](https://github.com/talos-systems/talos/issues/325))
* update go packages ([#324](https://github.com/talos-systems/talos/issues/324))
* update conform config ([#322](https://github.com/talos-systems/talos/issues/322))
* use buildkitd for builds ([#320](https://github.com/talos-systems/talos/issues/320))
* use the toolchain for go builds ([#317](https://github.com/talos-systems/talos/issues/317))
* improve build time ([#315](https://github.com/talos-systems/talos/issues/315))
* add nolint annotation ([#313](https://github.com/talos-systems/talos/issues/313))
* remove redundant tasks in build ([#311](https://github.com/talos-systems/talos/issues/311))
* use the TAG var for container tags ([#305](https://github.com/talos-systems/talos/issues/305))
* enforce commit and license policies ([#304](https://github.com/talos-systems/talos/issues/304))
* **conformance:** fix output path of sonobuoy ([#329](https://github.com/talos-systems/talos/issues/329))

### Docs

* update README ([#302](https://github.com/talos-systems/talos/issues/302))

### Feat

* upgrade Kubernetes to v1.13.2 ([#319](https://github.com/talos-systems/talos/issues/319))
* upgrade containerd to v1.2.2 ([#318](https://github.com/talos-systems/talos/issues/318))
* use musl libc ([#316](https://github.com/talos-systems/talos/issues/316))
* import core service containers from local store ([#309](https://github.com/talos-systems/talos/issues/309))
* **osctl:** add stats command ([#314](https://github.com/talos-systems/talos/issues/314))
* **osctl:** output namespace ([#312](https://github.com/talos-systems/talos/issues/312))

### Fix

* use version tag for container tags ([#308](https://github.com/talos-systems/talos/issues/308))
* raw image output ([#307](https://github.com/talos-systems/talos/issues/307))
* **init:** update probe for NVMe ([#323](https://github.com/talos-systems/talos/issues/323))
* **osctl:** compile static binary with CGO enabeld ([#328](https://github.com/talos-systems/talos/issues/328))

### Refactor

* use containerd exported defaults ([#310](https://github.com/talos-systems/talos/issues/310))


<a name="v0.1.0-alpha.15"></a>
## [v0.1.0-alpha.15](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.14...v0.1.0-alpha.15) (2019-01-02)

### Chore

* prepare v0.1.0-alpha.15 release ([#301](https://github.com/talos-systems/talos/issues/301))
* remove unneeded files from initramfs ([#299](https://github.com/talos-systems/talos/issues/299))
* use the existing docker install for AMI builds ([#297](https://github.com/talos-systems/talos/issues/297))
* use buildkit for builds ([#295](https://github.com/talos-systems/talos/issues/295))
* remove toolchain and kernel builds ([#290](https://github.com/talos-systems/talos/issues/290))
* ignore linting error ([#285](https://github.com/talos-systems/talos/issues/285))
* prepare v0.1.0-alpha.14 release ([#277](https://github.com/talos-systems/talos/issues/277))
* reduce xz compression level of initramfs ([#252](https://github.com/talos-systems/talos/issues/252))
* pin protoc-gen-go to v1.2.0 ([#235](https://github.com/talos-systems/talos/issues/235))
* add gpt scope ([#239](https://github.com/talos-systems/talos/issues/239))
* add helper script for pushing images ([#233](https://github.com/talos-systems/talos/issues/233))
* **ci:** add pod resource requests and limits ([#247](https://github.com/talos-systems/talos/issues/247))
* **ci:** fix build script ([#248](https://github.com/talos-systems/talos/issues/248))

### Docs

* update docs for new year ([#300](https://github.com/talos-systems/talos/issues/300))

### Feat

* udevd service ([#231](https://github.com/talos-systems/talos/issues/231))
* atomic partition table operations ([#234](https://github.com/talos-systems/talos/issues/234))
* use Containerd as CRI ([#292](https://github.com/talos-systems/talos/issues/292))
* upgrade Kubernetes to v1.13.1 ([#291](https://github.com/talos-systems/talos/issues/291))
* add filesystem probing library ([#298](https://github.com/talos-systems/talos/issues/298))
* **image:** build AMI with random.trust_cpu=on ([#287](https://github.com/talos-systems/talos/issues/287))
* **init:** reboot node on panic ([#284](https://github.com/talos-systems/talos/issues/284))
* **init:** add label and force options for xfs ([#244](https://github.com/talos-systems/talos/issues/244))
* **init:** add support for installing to a device ([#225](https://github.com/talos-systems/talos/issues/225))
* **init:** enable PSP admission plugin ([#230](https://github.com/talos-systems/talos/issues/230))
* **init:** create CNI mounts ([#226](https://github.com/talos-systems/talos/issues/226))
* **initramfs:** retry userdata download ([#283](https://github.com/talos-systems/talos/issues/283))
* **initramfs:** API for creating new partition tables ([#227](https://github.com/talos-systems/talos/issues/227))
* **kernel:** upgrade Linux to v4.19.10 ([#293](https://github.com/talos-systems/talos/issues/293))

### Fix

* disable AlwaysPullImages admission plugin ([#273](https://github.com/talos-systems/talos/issues/273))
* symlink kubernetes libexec directory ([#294](https://github.com/talos-systems/talos/issues/294))
* **gpt:** do not inform kernel of partition when writing ([#237](https://github.com/talos-systems/talos/issues/237))
* **image:** install gzip ([#272](https://github.com/talos-systems/talos/issues/272))
* **init:** don't create the EncryptionConfig if it exists ([#282](https://github.com/talos-systems/talos/issues/282))
* **init:** use PARTLABEL to identity Talos block devices ([#238](https://github.com/talos-systems/talos/issues/238))
* **init:** revert e94095b and fix bad attribute lookups ([#274](https://github.com/talos-systems/talos/issues/274))
* **init:** use the correct blkid lookup values ([#243](https://github.com/talos-systems/talos/issues/243))
* **init:** no memory limit for container runtime ([#289](https://github.com/talos-systems/talos/issues/289))
* **init:** address linter errors ([#251](https://github.com/talos-systems/talos/issues/251))
* **init:** unmount / last ([#249](https://github.com/talos-systems/talos/issues/249))
* **init:** use smaller default install sizes ([#240](https://github.com/talos-systems/talos/issues/240))
* **init:** use text/template ([#228](https://github.com/talos-systems/talos/issues/228))
* **initramfs:** minor fixes for booting from bare metal ([#241](https://github.com/talos-systems/talos/issues/241))
* **initramfs:** fix bare metal install ([#245](https://github.com/talos-systems/talos/issues/245))
* **initramfs:** fix hardcoded version ([#275](https://github.com/talos-systems/talos/issues/275))
* **initramfs:** imports ([#276](https://github.com/talos-systems/talos/issues/276))
* **initramfs:** fix printf statement ([#250](https://github.com/talos-systems/talos/issues/250))
* **kernel:** add missing kernel config options ([#236](https://github.com/talos-systems/talos/issues/236))

### Refactor

* use reserved ports for services ([#288](https://github.com/talos-systems/talos/issues/288))
* **image:** Changing rootfs from xz -> gz ([#232](https://github.com/talos-systems/talos/issues/232))
* **init:** DRY symlink creation ([#280](https://github.com/talos-systems/talos/issues/280))
* **init:** remove unnecessary unmount/mount ([#246](https://github.com/talos-systems/talos/issues/246))

### Style

* run gofmt ([#229](https://github.com/talos-systems/talos/issues/229))


<a name="v0.1.0-alpha.14"></a>
## [v0.1.0-alpha.14](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.13...v0.1.0-alpha.14) (2018-11-21)

### Feat

* **init:** add calico support ([#223](https://github.com/talos-systems/talos/issues/223))
* **init:** service env var option ([#219](https://github.com/talos-systems/talos/issues/219))
* **init:** add VMware support ([#200](https://github.com/talos-systems/talos/issues/200))
* **init:** log to /dev/kmsg ([#214](https://github.com/talos-systems/talos/issues/214))
* **kernel:** add raw iptables support ([#222](https://github.com/talos-systems/talos/issues/222))
* **kernel:** add igb and ixgb drivers ([#221](https://github.com/talos-systems/talos/issues/221))
* **kernel:** add low level SCSI support ([#215](https://github.com/talos-systems/talos/issues/215))
* **kernel:** add vmxnet3 support ([#213](https://github.com/talos-systems/talos/issues/213))

### Fix

* **hack:** remove privileged options from debug manifest ([#224](https://github.com/talos-systems/talos/issues/224))
* **init:** retry mounts ([#220](https://github.com/talos-systems/talos/issues/220))
* **init:** log to kmsg after /dev is mounted ([#218](https://github.com/talos-systems/talos/issues/218))
* **init:** ensure VMware user data is not empty ([#217](https://github.com/talos-systems/talos/issues/217))
* **init:** avoid kernel panic on recover ([#216](https://github.com/talos-systems/talos/issues/216))
* **init:** allow custom image for kubeadm ([#212](https://github.com/talos-systems/talos/issues/212))

### Refactor

* rename project to Talos ([#211](https://github.com/talos-systems/talos/issues/211))


<a name="v0.1.0-alpha.13"></a>
## [v0.1.0-alpha.13](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.12...v0.1.0-alpha.13) (2018-11-15)

### Chore

* prepare v0.1.0-alpha.13 release ([#210](https://github.com/talos-systems/talos/issues/210))
* run tests before linting ([#206](https://github.com/talos-systems/talos/issues/206))
* update artifact destination ([#202](https://github.com/talos-systems/talos/issues/202))
* build xfsprogs before binaries ([#197](https://github.com/talos-systems/talos/issues/197))
* fix node selector ([#185](https://github.com/talos-systems/talos/issues/185))

### Docs

* add Xen example ([#193](https://github.com/talos-systems/talos/issues/193))
* fix typos ([#188](https://github.com/talos-systems/talos/issues/188))
* improve configuration documentation ([#186](https://github.com/talos-systems/talos/issues/186))

### Feat

* embed the kubeadm config ([#205](https://github.com/talos-systems/talos/issues/205))
* ***:** upgrade Containerd to v1.2.0 ([#190](https://github.com/talos-systems/talos/issues/190))
* ***:** upgrade Kubernetes to v1.13.0-alpha.3 ([#189](https://github.com/talos-systems/talos/issues/189))
* **init:** add NoCloud user-data support ([#209](https://github.com/talos-systems/talos/issues/209))
* **init:** enforce CIS requirements ([#198](https://github.com/talos-systems/talos/issues/198))
* **init:** enforce use of hyperkube and Kubernetes version ([#207](https://github.com/talos-systems/talos/issues/207))
* **kernel:** upgrade Linux to v4.19.1 ([#192](https://github.com/talos-systems/talos/issues/192))
* **kernel:** add virtio support ([#208](https://github.com/talos-systems/talos/issues/208))
* **rootfs:** upgrade crictl to v1.12.0 ([#191](https://github.com/talos-systems/talos/issues/191))

### Fix

* **hack:** add /etc/kubernetes to CIS benchmark jobs ([#199](https://github.com/talos-systems/talos/issues/199))
* **image:** VMDK generation ([#204](https://github.com/talos-systems/talos/issues/204))
* **init:** node join ([#195](https://github.com/talos-systems/talos/issues/195))
* **init:** use kubeadm experimental-control-plane ([#194](https://github.com/talos-systems/talos/issues/194))
* **osctl:** build Linux binary with CGO ([#196](https://github.com/talos-systems/talos/issues/196))
* **osctl:** nil pointer when injecting kubernetes PKI ([#187](https://github.com/talos-systems/talos/issues/187))


<a name="v0.1.0-alpha.12"></a>
## [v0.1.0-alpha.12](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.11...v0.1.0-alpha.12) (2018-11-01)

### Chore

* prepare v0.1.0-alpha.12 release ([#184](https://github.com/talos-systems/talos/issues/184))
* run CI jobs on dedicated nodes ([#174](https://github.com/talos-systems/talos/issues/174))
* ignore .vscode ([#175](https://github.com/talos-systems/talos/issues/175))

### Docs

* add rendered files ([#183](https://github.com/talos-systems/talos/issues/183))
* improve search result previews ([#182](https://github.com/talos-systems/talos/issues/182))
* add search ([#181](https://github.com/talos-systems/talos/issues/181))
* improve sidebar style ([#180](https://github.com/talos-systems/talos/issues/180))
* fix CDN to use https ([#179](https://github.com/talos-systems/talos/issues/179))
* add navbar to theme ([#178](https://github.com/talos-systems/talos/issues/178))
* update CSS for small screens ([#177](https://github.com/talos-systems/talos/issues/177))
* add section navigation to theme ([#176](https://github.com/talos-systems/talos/issues/176))
* add documention ([#158](https://github.com/talos-systems/talos/issues/158))

### Feat

* add blockd service ([#172](https://github.com/talos-systems/talos/issues/172))
* ***:** upgrade Kubernetes to v1.13.0-alpha.2 ([#173](https://github.com/talos-systems/talos/issues/173))


<a name="v0.1.0-alpha.11"></a>
## [v0.1.0-alpha.11](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.10...v0.1.0-alpha.11) (2018-10-17)

### Chore

* prepare v0.1.0-alpha.11 release ([#171](https://github.com/talos-systems/talos/issues/171))
* tag and push docker image with semver ([#165](https://github.com/talos-systems/talos/issues/165))
* update CIS kube-bench version to 1.11 ([#161](https://github.com/talos-systems/talos/issues/161))
* remove unused go module files ([#159](https://github.com/talos-systems/talos/issues/159))
* **ci:** add brigade configuration ([#166](https://github.com/talos-systems/talos/issues/166))
* **image:** upgrade Packer to v1.3.1 ([#163](https://github.com/talos-systems/talos/issues/163))

### Feat

* ***:** upgrade Kubernetes to v1.13.0-alpha.1 ([#162](https://github.com/talos-systems/talos/issues/162))
* **ami:** enable ena support ([#164](https://github.com/talos-systems/talos/issues/164))
* **init:** mount partitions dynamically ([#169](https://github.com/talos-systems/talos/issues/169))
* **kernel:** enable NVMe support ([#170](https://github.com/talos-systems/talos/issues/170))

### Fix

* **image:** align VERSION env var with pkg/version ([#168](https://github.com/talos-systems/talos/issues/168))
* **init:** disable megacheck until it gains module support ([#167](https://github.com/talos-systems/talos/issues/167))
* **init:** add /dev and /usr/libexec/kubernetes to docker service ([#160](https://github.com/talos-systems/talos/issues/160))
* **kernel:** remove slub_debug kernel param ([#157](https://github.com/talos-systems/talos/issues/157))


<a name="v0.1.0-alpha.10"></a>
## [v0.1.0-alpha.10](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.9...v0.1.0-alpha.10) (2018-10-13)

### Chore

* ***:** update conform commands ([#150](https://github.com/talos-systems/talos/issues/150))
* **conformance:** add usage to sonobuoy script ([#156](https://github.com/talos-systems/talos/issues/156))
* **conformance:** remove old conformance tasks ([#155](https://github.com/talos-systems/talos/issues/155))
* **conformance:** sonobuoy script and kube-bench job ([#154](https://github.com/talos-systems/talos/issues/154))

### Feat

* ***:** upgrade all core components ([#153](https://github.com/talos-systems/talos/issues/153))
* **kernel:** configure Kernel Self Protection Project recommendations ([#152](https://github.com/talos-systems/talos/issues/152))


<a name="v0.1.0-alpha.9"></a>
## [v0.1.0-alpha.9](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.8...v0.1.0-alpha.9) (2018-09-20)

### Chore

* ***:** upgrade Go to v1.11.0 ([#145](https://github.com/talos-systems/talos/issues/145))

### Feat

* ***:** run system services via containerd ([#149](https://github.com/talos-systems/talos/issues/149))
* **kernel:** upgrade Linux to v4.18.5 ([#147](https://github.com/talos-systems/talos/issues/147))

### Fix

* **init:** address linter error ([#146](https://github.com/talos-systems/talos/issues/146))

### Refactor

* **initramfs:** rename rotd to trustd ([#148](https://github.com/talos-systems/talos/issues/148))


<a name="v0.1.0-alpha.8"></a>
## [v0.1.0-alpha.8](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.7...v0.1.0-alpha.8) (2018-08-28)

### Feat

* ***:** HA control plane ([#144](https://github.com/talos-systems/talos/issues/144))
* ***:** list and restart processes ([#141](https://github.com/talos-systems/talos/issues/141))
* **kernel:** upgrade Linux to v4.17.15 ([#140](https://github.com/talos-systems/talos/issues/140))
* **osd:** node reset and reboot ([#142](https://github.com/talos-systems/talos/issues/142))


<a name="v0.1.0-alpha.7"></a>
## [v0.1.0-alpha.7](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.6...v0.1.0-alpha.7) (2018-08-10)

### Chore

* **image:** push docker images ([#131](https://github.com/talos-systems/talos/issues/131))

### Feat

* ***:** upgrade Kubernetes to v1.11.2 ([#139](https://github.com/talos-systems/talos/issues/139))
* **conformance:** add quick mode config ([#129](https://github.com/talos-systems/talos/issues/129))
* **conformance:** add conformance image ([#126](https://github.com/talos-systems/talos/issues/126))
* **hack:**  add CIS Kubernetes Benchmark script ([#134](https://github.com/talos-systems/talos/issues/134))
* **hack:** use ubuntu 18.04 image in debug pod ([#135](https://github.com/talos-systems/talos/issues/135))
* **image:** make AMI regions a variable ([#137](https://github.com/talos-systems/talos/issues/137))
* **init:** add file creation option ([#132](https://github.com/talos-systems/talos/issues/132))
* **init:** debug option ([#138](https://github.com/talos-systems/talos/issues/138))
* **initramfs:** check for self-hosted-kube-apiserver label ([#130](https://github.com/talos-systems/talos/issues/130))
* **kernel:** upgrade Linux to v4.17.10 ([#128](https://github.com/talos-systems/talos/issues/128))

### Fix

* **init:** read kubeadm env file ([#136](https://github.com/talos-systems/talos/issues/136))
* **init:** make /etc/hosts writable ([#125](https://github.com/talos-systems/talos/issues/125))
* **initramfs:** align go tests with upstream change ([#133](https://github.com/talos-systems/talos/issues/133))


<a name="v0.1.0-alpha.6"></a>
## [v0.1.0-alpha.6](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.5...v0.1.0-alpha.6) (2018-07-23)

### Feat

* ***:** upgrade Kubernetes to v1.11.1 ([#123](https://github.com/talos-systems/talos/issues/123))
* ***:** run the kubelet in a container ([#122](https://github.com/talos-systems/talos/issues/122))
* ***:** add a debug pod manifest ([#120](https://github.com/talos-systems/talos/issues/120))
* **image:** generate image ([#114](https://github.com/talos-systems/talos/issues/114))
* **initramfs:** set the platform explicitly ([#124](https://github.com/talos-systems/talos/issues/124))
* **initramfs:** rewrite user data ([#121](https://github.com/talos-systems/talos/issues/121))

### Fix

* **rootfs:** don't remove the docker binary ([#119](https://github.com/talos-systems/talos/issues/119))

### Refactor

* ***:** move source code into src directory ([#118](https://github.com/talos-systems/talos/issues/118))


<a name="v0.1.0-alpha.5"></a>
## [v0.1.0-alpha.5](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.4...v0.1.0-alpha.5) (2018-07-02)

### Chore

* ***:** update generated files ([#110](https://github.com/talos-systems/talos/issues/110))
* ***:** use https wherever possible in source URLs ([#109](https://github.com/talos-systems/talos/issues/109))
* **initramfs:** disable cgo for osctl ([#113](https://github.com/talos-systems/talos/issues/113))

### Core

* **generate:** use first unused loopback device ([#112](https://github.com/talos-systems/talos/issues/112))

### Feat

* **init:** configurable kubelet arguments ([#99](https://github.com/talos-systems/talos/issues/99))
* **init:** platform discovery ([#101](https://github.com/talos-systems/talos/issues/101))
* **initramfs:** Kubernetes API reverse proxy ([#107](https://github.com/talos-systems/talos/issues/107))
* **kernel:** enable Ceph ([#105](https://github.com/talos-systems/talos/issues/105))
* **rootfs:** upgrade Docker to v17.03.2-ce ([#111](https://github.com/talos-systems/talos/issues/111))
* **rootfs:** install cut ([#106](https://github.com/talos-systems/talos/issues/106))
* **rootfs:** upgrade Kubernetes to v1.11.0-beta.1 ([#104](https://github.com/talos-systems/talos/issues/104))

### Fix

* ***:** create build directory ([#108](https://github.com/talos-systems/talos/issues/108))
* ***:** field tag should be yaml instead of json ([#100](https://github.com/talos-systems/talos/issues/100))


<a name="v0.1.0-alpha.4"></a>
## [v0.1.0-alpha.4](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.3...v0.1.0-alpha.4) (2018-05-19)

### Chore

* ***:** build AMI ([#83](https://github.com/talos-systems/talos/issues/83))
* **initramfs:** update generated code ([#91](https://github.com/talos-systems/talos/issues/91))

### Feat

* ***:** upgrade kubernetes to v1.11.0-beta.0 ([#92](https://github.com/talos-systems/talos/issues/92))
* ***:** osctl configuration file ([#90](https://github.com/talos-systems/talos/issues/90))
* ***:** dynamic resolv.conf ([#86](https://github.com/talos-systems/talos/issues/86))
* ***:** add version command ([#85](https://github.com/talos-systems/talos/issues/85))
* **init:** verify EC2 PKCS7 signature ([#84](https://github.com/talos-systems/talos/issues/84))

### Fix

* ***:** use commit SHA on master and tag name on tags ([#98](https://github.com/talos-systems/talos/issues/98))
* ***:** force the kernel to reread partition table ([#88](https://github.com/talos-systems/talos/issues/88))
* **init:** conditionally set version in /etc/os-release ([#97](https://github.com/talos-systems/talos/issues/97))
* **init:** use /proc/net/pnp as resolv.conf ([#87](https://github.com/talos-systems/talos/issues/87))
* **initramfs:** escape double quotes ([#96](https://github.com/talos-systems/talos/issues/96))
* **initramfs:** quote -X flag ([#95](https://github.com/talos-systems/talos/issues/95))
* **initramfs:** invalid reference to template variable ([#94](https://github.com/talos-systems/talos/issues/94))
* **initramfs:** build variables ([#93](https://github.com/talos-systems/talos/issues/93))

### Test

* **kernel:** runc check-config.sh ([#82](https://github.com/talos-systems/talos/issues/82))


<a name="v0.1.0-alpha.3"></a>
## [v0.1.0-alpha.3](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.2...v0.1.0-alpha.3) (2018-05-14)

### Feat

* ***:** automate signed certificates ([#81](https://github.com/talos-systems/talos/issues/81))
* ***:** raw kubeadm configuration in user data ([#79](https://github.com/talos-systems/talos/issues/79))
* **init:** don't print kubeadm token ([#74](https://github.com/talos-systems/talos/issues/74))
* **kernel:** compile with Linux guest support ([#75](https://github.com/talos-systems/talos/issues/75))

### Fix

* **generate:** use xvda instead of sda ([#77](https://github.com/talos-systems/talos/issues/77))
* **init:** bad variable name and missing package ([#78](https://github.com/talos-systems/talos/issues/78))

### Refactor

* ***:** move gRPC service to dedicated binary ([#73](https://github.com/talos-systems/talos/issues/73))
* **init:** small changes to improve readability ([#76](https://github.com/talos-systems/talos/issues/76))


<a name="v0.1.0-alpha.2"></a>
## [v0.1.0-alpha.2](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.1...v0.1.0-alpha.2) (2018-05-08)

### Feat

* ***:** upgrade Kubernetes to v1.10.2 ([#61](https://github.com/talos-systems/talos/issues/61))
* **generate:** set RAW disk sizes dynamically ([#71](https://github.com/talos-systems/talos/issues/71))
* **init:** gRPC with mutual TLS authentication ([#64](https://github.com/talos-systems/talos/issues/64))
* **rootfs:** upgrade CRI-O to v1.10.1 ([#70](https://github.com/talos-systems/talos/issues/70))

### Refactor

* ***:** address linter errors and warnings ([#69](https://github.com/talos-systems/talos/issues/69))


<a name="v0.1.0-alpha.1"></a>
## [v0.1.0-alpha.1](https://github.com/talos-systems/talos/compare/v0.1.0-alpha.0...v0.1.0-alpha.1) (2018-04-19)

### Chore

* **generate:** reduce the size of artifacts ([#52](https://github.com/talos-systems/talos/issues/52))

### Feat

* ***:** docker as an optional container runtime ([#57](https://github.com/talos-systems/talos/issues/57))
* ***:** upgrade to Kubernetes v1.10.1 ([#50](https://github.com/talos-systems/talos/issues/50))
* **generate:** enable kernel logging ([#58](https://github.com/talos-systems/talos/issues/58))
* **kernel:** use LTS kernel v4.14.34 ([#48](https://github.com/talos-systems/talos/issues/48))

### Fix

* ***:** generate /etc/hosts and /etc/resolv.conf ([#54](https://github.com/talos-systems/talos/issues/54))
* **init:** enable hierarchical accounting and reclaim ([#59](https://github.com/talos-systems/talos/issues/59))
* **init:** remove unused code ([#56](https://github.com/talos-systems/talos/issues/56))
* **init:** missing parameter ([#55](https://github.com/talos-systems/talos/issues/55))
* **init:** printf formatting ([#51](https://github.com/talos-systems/talos/issues/51))
* **init:** switch_root implementation ([#49](https://github.com/talos-systems/talos/issues/49))

### Refactor

* **init:** use /root for new root path ([#53](https://github.com/talos-systems/talos/issues/53))


<a name="v0.1.0-alpha.0"></a>
## v0.1.0-alpha.0 (2018-04-02)

### Chore

* **rootfs:** cleanup include and share directories ([#28](https://github.com/talos-systems/talos/issues/28))

### Feat

* ***:** update to linux 4.15.13 ([#30](https://github.com/talos-systems/talos/issues/30))
* ***:** mount ROOT partition as RO ([#11](https://github.com/talos-systems/talos/issues/11))
* ***:** use CRI-O as the container runtime ([#12](https://github.com/talos-systems/talos/issues/12))
* ***:** update Kubernetes to v1.10.0-rc.1 ([#25](https://github.com/talos-systems/talos/issues/25))
* ***:** update Kubernetes to v1.10.0 ([#26](https://github.com/talos-systems/talos/issues/26))
* ***:** enable IPVS ([#42](https://github.com/talos-systems/talos/issues/42))
* ***:** initial implementation ([#2](https://github.com/talos-systems/talos/issues/2))
* **init:** provide and endpoint for getting logs of running processes ([#9](https://github.com/talos-systems/talos/issues/9))
* **init:** add node join functionality ([#38](https://github.com/talos-systems/talos/issues/38))
* **init:** set kubelet log level to 4 ([#13](https://github.com/talos-systems/talos/issues/13))
* **init:** use CoreDNS by default ([#39](https://github.com/talos-systems/talos/issues/39))
* **init:** basic process managment ([#6](https://github.com/talos-systems/talos/issues/6))
* **init:** user data ([#17](https://github.com/talos-systems/talos/issues/17))
* **kernel:** enable nf_tables and ebtables modules ([#41](https://github.com/talos-systems/talos/issues/41))
* **rootfs:** upgrade cri-o and cri-tools ([#35](https://github.com/talos-systems/talos/issues/35))

### Fix

* **init:** address crio errors and warns ([#40](https://github.com/talos-systems/talos/issues/40))
* **init:** make log handling non-blocking ([#37](https://github.com/talos-systems/talos/issues/37))
* **init:** don't create CRI-O CNI configurations ([#36](https://github.com/talos-systems/talos/issues/36))
* **init:** typo in service subnet field; pin version of Kubernetes ([#10](https://github.com/talos-systems/talos/issues/10))
* **rootfs:** install conntrack ([#27](https://github.com/talos-systems/talos/issues/27))

### Refactor

* **init:** use recover builtin to avoid kernel panics ([#15](https://github.com/talos-systems/talos/issues/15))
* **rootfs:** install conntrack-tools earlier in the pipeline ([#31](https://github.com/talos-systems/talos/issues/31))

