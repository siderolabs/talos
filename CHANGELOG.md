
<a name="v0.7.0-alpha.3"></a>
## [v0.7.0-alpha.3](https://github.com/talos-systems/talos/compare/v0.7.0-alpha.2...v0.7.0-alpha.3) (2020-09-25)

### Chore

* fix edge push
* fix docker login
* fix docker login
* migrate to ghcr.io
* push edge releases on successful nightly integration

### Docs

* add note around link-local addressing
* add ghcr.io to the registry cache docs
* add v0.7 docs

### Feat

* bump default resource limits for `talosctl cluster create`
* add default install image
* add images command

### Fix

* update one more places which had stale reference for constants
* update the docs to fix the lint-markdown
* use images package in integration tests
* move installer image variables out of machinery
* enable --removable options for GRUB
* retry image pulling, stop on 404, no duplicate pulls


<a name="v0.7.0-alpha.2"></a>
## [v0.7.0-alpha.2](https://github.com/talos-systems/talos/compare/v0.7.0-alpha.1...v0.7.0-alpha.2) (2020-09-18)

### Chore

* update ntp time headers
* upgrade Go to 1.15.1
* remove extra COPY from rootfs

### Docs

* add recommneded settings in overview
* update upgrade guide with `talosctl upgrade-k8s`
* update 0.6 links

### Feat

* ugrade Linux kernel to 5.8.10
* allow for link local networking
* use architecture-specific image for core k8s components
* update Flannel to 0.12, support for arm64
* upgrade kubernetes to 1.19.1
* implement command `talosctl upgrade-k8s`
* use latest packages
* upgrade runc to v1.0.0-rc92
* upgrade containerd to v1.4.0
* remove ISO support

### Fix

* address node package update
* validate cluster endpoint
* improve error message on empty config
* gracefully handle invalid interfaces in bond
* set environment variable for etcd on arm64
* don't enforce k8s version in `talosctl cluster create` by default
* tell grub to use console output
* update vmware image and platform
* don't abort reboot sequence on bootloader meta failure
* default endpoint to 127.0.0.1 for Docker/OS X
* remove udevd debug flag
* update permissions for directories and files created via extraFiles
* allow static pod files

### Refactor

* garbage collect unused constants
* deduplicate packages version in Dockerfile

### Release

* **v0.7.0-alpha.2:** prepare release

### Test

* implement API for QEMU VM provisioner
* re-enable Cilium e2e upgrade test
* verify kubernetes control plane upgrade in provision tests
* add e2e test to the provision (upgrade) tests


<a name="v0.7.0-alpha.1"></a>
## [v0.7.0-alpha.1](https://github.com/talos-systems/talos/compare/v0.7.0-alpha.0...v0.7.0-alpha.1) (2020-09-02)

### Chore

* update k8s modules to 1.19 final version
* upgrade Go to 1.14.8
* drop vmlinux from assets
* add a method to merge Talos client config
* bump next version to v0.6.0-beta.2
* update machinery version in go.mod
* update node.js dependencies

### Docs

* graduate v0.6 docs
* add Kubernetes upgrade guide
* add reset doc
* add QEMU provisioner documentation

### Feat

* add grub bootloader
* upgrade etcd to 3.4.12
* provide option to run Talos under UEFI in QEMU
* update linux to 5.8.5
* update kubernetes to v1.19.0
* make boostrap via API default choice in talosctl cluster create
* upgrade Linux to v5.7.15

### Fix

* change apid container image name to expected value
* add syslinux to create ISO
* pass config via stdin
* handle bootkube recover correctly, support recovery from etcd

### Refactor

* move udevadm trigger/settle to udevd healthcheck
* extract packages loadbalancer and retry
* extract cluster bootstrapper via API as common component

### Release

* **v0.7.0-alpha.1:** prepare release
* **v0.7.0-alpha.1:** prepare release

### Test

* determine reboots using boot id
* add support for PXE nodes in qemu provision library

### BREAKING CHANGE


Single node upgrades will fail in this change. This
will also break the A/B fallback setup since this version introduces
an entirely new partition scheme, that any fallback will not know about.
We plan on addressing these issues in a follow up change.

