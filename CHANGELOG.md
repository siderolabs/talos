<a name="v0.6.0-beta.0"></a>

## [v0.6.0-beta.0](https://github.com/talos-systems/talos/compare/v0.6.0-alpha.6...v0.6.0-beta.0) (2020-08-03)

### Chore

- bump elliptic from 6.5.2 to 6.5.3 in /docs/website
- add aliases to some `talosctl` commands
- use qemu instead of firecracker in CI
- really mount /tmp in CI as tmpfs
- mount `/tmp` in CI to the build steps
- add release notes

### Feat

- add dynamic config decoder
- taint master nodes with `NoSchedule` taint
- upgrade Kubernetes to v1.19.0-rc.3
- qemu provisioner
- pull in kernel with fuse support

### Fix

- update AMI link to latest
- workaround edge case for etcd re-injection on bootstrap
- update status when adjusting the time
- fail ntpd service if initial time sync fails
- bump timeouts
- generate admin kubeconfig with default namespace

### Refactor

- make `pkg/config` not rely on `machined/../internal/runtime`

### Release

- **v0.6.0-beta.0:** prepare release

### Test

- use registry mirrors in CI
- destroy clusters in e2e tests (qemu/firecracker)
- bump timeout for upgrade tests
- update qemu/firecracker provisioners
- upgrade versions the upgrade tests are operating on
- provide node discovery for cli tests via kubectl
- remove apid load balancer for firecracker

<a name="v0.6.0-alpha.6"></a>

## [v0.6.0-alpha.6](https://github.com/talos-systems/talos/compare/v0.6.0-alpha.5...v0.6.0-alpha.6) (2020-07-27)

### Chore

- set default CIDRs
- use outer docker as buildkit instance
- upgrade pkgs and tools for Go 1.14.6
- use Kubernetes pipelines
- bump lodash from 4.17.15 to 4.17.19 in /docs/website
- extract loadbalancer, network, crashdup and process from firecracker
- initial extraction of base vm provisioner
- move inmemhttp from firecracker provisioner to internal/pkg/
- update module dependencies
- update golangci-lint to 1.28.3
- upgrade Go to 1.14.5
- update clusterctl for CI testing

### Docs

- use latest talosctl download link
- update worker creation flags for azure docs

### Feat

- force nodes to be set in `talosctl` commands using the API
- upgrade etcd to 3.3.22 version
- make partitions on additional disk without size occupy full disk
- implement talosctl dashboard command
- implement server-side API for cluster health checks
- upgrade Kubernetes to v1.19.0-rc.0

### Fix

- log interface on validation error
- skip removing CRI state when doing upgrade with preserve
- skip vmware platform for !amd64
- log messages properly when sequence/phase/task fails
- ignore sequence lock errors in machined
- wrap errors in upgrade API handler
- update container name in docker crashdump

### Refactor

- use `humanize.Bytes` everywhere

### Release

- **v0.6.0-alpha.6:** prepare release

### Test

- add an option to bind docker to specific host IP
- fix racy test ReaderNoFollow
- provider correct installer kernel args for firecracker

<a name="v0.6.0-alpha.5"></a>

## [v0.6.0-alpha.5](https://github.com/talos-systems/talos/compare/v0.6.0-alpha.4...v0.6.0-alpha.5) (2020-07-13)

### Chore

- update meeting links
- wait for resource deletion in sonobuoy
- cleanup sonobuoy after failed attempts
- enable 'testpackage' linter
- make default pipeline run shorter integration test
- enable godot linter

### Docs

- update firecracker for new home of tc-redirect-tap plugin
- digital rebar docs

### Feat

- add names to tasks and phases
- merge mode in talosctl kubeconfig
- print crash dump in `talosctl cluster create` on failure
- uncordon nodes automatically on boot
- add round-robin LB policy to Talos client by default
- implement API access to event history
- implement service events
- upgrade runc to v1.0.0-rc90
- upgrade Linux to v5.7.7
- upgrade containerd to v1.3.6
- add /system directory

### Fix

- improve node uncordon tasks
- update the control plane cluster health check
- update timeouts on service startup to match boot timeout
- implement Unload() for services to make sure bootkube runs always
- print correct sequence/task duration
- provide default DNS domain to talosctl cluster create
- report the correct containerd version

### Refactor

- merge osd into machined

### Release

- **v0.6.0-alpha.5:** prepare release

### Test

- workaround famous flaky Containerd.RunTwice test
- update events test with more flow control
- update tests for `pkg/follow` to be less time-dependent
- update init node check in reset API tests
- fix cli tests after load-balancing got enabled
- fix sonobuoy delete
- resolve old TODO item
- run integration pipeline nightly
- stabilize race unit-tests (circular, events)
- run `e2e-firecracker-short` for default pipeline only
- add short integration test with custom CNI

<a name="v0.6.0-alpha.4"></a>

## [v0.6.0-alpha.4](https://github.com/talos-systems/talos/compare/v0.6.0-alpha.3...v0.6.0-alpha.4) (2020-06-30)

### Chore

- enable nolintlint linter
- bring back tmp volume shared from e2e-docker to CAPI steps
- stop mounting /tmp for the build pipeline
- upgrade golangci-lint to 1.27
- output where we are pulling configs for each platform
- update kernel to support CONFIG_CRYPTO_USER_API_HASH
- sign the drone file

### Docs

- add local registry cache documentation
- update firecracker with one more CNI plugin
- specs added
- specs added
- extend contribution doc
- extend contribution doc

### Feat

- implement circular buffer for system logs
- allow ability to create dummy nics

### Fix

- use kubernetes version in config generator
- make installer re-read partition table before formatting
- attempt to pull machine config from mounted disk in azure
- isolate kubelet /run directory
- check if machine networking is nil
- detect failed bootkube run properly
- delete manifests dir on bootkube failure

### Release

- **v0.6.0-alpha.4:** prepare release

### Test

- fix and improve reboot/reset tests
- default to using the bootstrap API

<a name="v0.6.0-alpha.3"></a>

## [v0.6.0-alpha.3](https://github.com/talos-systems/talos/compare/v0.6.0-alpha.2...v0.6.0-alpha.3) (2020-06-17)

### Chore

- run provision tests in parallel
- use neutral terminology

### Feat

- add rollback command
- add open-iscsi
- update linux kernel (with 32 bit support) and talos pkgs for v0.6
- allow recovery at all times

### Fix

- detect if partition table is missing
- revert default boot properly
- allow for using /dev/disk/\* symlinks
- skip services when in container mode
- activate logical volumes
- update LVM2

### Release

- **v0.6.0-alpha.3:** prepare release

<a name="v0.6.0-alpha.2"></a>

## [v0.6.0-alpha.2](https://github.com/talos-systems/talos/compare/v0.6.0-alpha.1...v0.6.0-alpha.2) (2020-06-10)

### Chore

- update provision test versions

### Docs

- add v0.6 docs
- add kernel options to firecracker reqs
- remove repeated component in the Arges architecture image
- add talosctl docs document
- fix a few minor styling issues

### Feat

- update kubernetes to 1.19.0-beta.1
- update k8s and sonobuoy versions
- add rollback API
- allow reset API at all times
- adjust time properly in timed via adjtime()

### Fix

- allow node names
- make services depend on timed
- correctly handle IPv6 address in apid

### Refactor

- implement LoggingManager as central log flow processor

### Release

- **v0.6.0-alpha.2:** prepare release

### Test

- fix race in some tests caused by `SetT`

<a name="v0.6.0-alpha.1"></a>

## [v0.6.0-alpha.1](https://github.com/talos-systems/talos/compare/v0.6.0-alpha.0...v0.6.0-alpha.1) (2020-05-27)

### Chore

- fix markdown lint
- upgrade Go to 1.14.3 and use toolchain for race detector
- replace underlying event implementation with single slice

### Docs

- make v0.5 docs the default
- fix markdown
- add metal overview diagram
- fix broken links in components pages (fixes [#2117](https://github.com/talos-systems/talos/issues/2117))
- add some information about Arges and expand the bare metal section a bit
- overview of talos components

### Feat

- add LVM2
- implement simplified client method to consume events
- upgrade Linux to v5.6.13

### Fix

- prevent panic on nil pointer in ServiceInfo method
- bump service wait to ten minutes
- allow all seccomp profile names
- wrap etcd address URLs with formatting

### Release

- **v0.6.0-alpha.1:** prepare release

### Test

- improve reboot/reset test resiliency against request timeouts
- update Talos versions for upgrade tests
