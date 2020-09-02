<a name="v0.7.0-alpha.1"></a>

## [v0.7.0-alpha.1](https://github.com/talos-systems/talos/compare/v0.7.0-alpha.0...v0.7.0-alpha.1) (2020-09-02)

### Chore

- update k8s modules to 1.19 final version
- upgrade Go to 1.14.8
- drop vmlinux from assets
- add a method to merge Talos client config
- bump next version to v0.6.0-beta.2
- update machinery version in go.mod
- update node.js dependencies

### Docs

- graduate v0.6 docs
- add Kubernetes upgrade guide
- add reset doc
- add QEMU provisioner documentation

### Feat

- add grub bootloader
- upgrade etcd to 3.4.12
- provide option to run Talos under UEFI in QEMU
- update linux to 5.8.5
- update kubernetes to v1.19.0
- make boostrap via API default choice in talosctl cluster create
- upgrade Linux to v5.7.15

### Fix

- change apid container image name to expected value
- add syslinux to create ISO
- pass config via stdin
- handle bootkube recover correctly, support recovery from etcd

### Refactor

- move udevadm trigger/settle to udevd healthcheck
- extract packages loadbalancer and retry
- extract cluster bootstrapper via API as common component

### Release

- **v0.7.0-alpha.1:** prepare release

### Test

- determine reboots using boot id
- add support for PXE nodes in qemu provision library

### BREAKING CHANGE

Single node upgrades will fail in this change. This
will also break the A/B fallback setup since this version introduces
an entirely new partition scheme, that any fallback will not know about.
We plan on addressing these issues in a follow up change.

<a name="v0.7.0-alpha.0"></a>

## [v0.7.0-alpha.0](https://github.com/talos-systems/talos/compare/v0.6.0-beta.1...v0.7.0-alpha.0) (2020-08-17)

### Chore

- re-import talos-systems/pkg/crypto/tls
- extract pkg/crypto as external module
- integrate importvet
- update capi CI manifests to use control planes
- update node dependencies
- update packages

### Docs

- fix download link

### Feat

- upgrade etcd to 3.4.10
- add persist flag to gen config

### Fix

- run health check for etcd service with Get API
- ignore eth0 interface in docker provisioner
- update e2e scripts to work with python3
- retry non-HTTP errors from API server
- update qemu launcher on arm64 to boot Talos properly

### Refactor

- move external API packages into `machinery/`
- rework `pkg/grpc/tls` to break dependency on `pkg/grpc/gen`
- extract `pkg/net` as `github.com/talos-systems/net`
- expose `provision` as public package
- remove structs from config provider
