
<a name="v0.8.0-alpha.1"></a>
## [v0.8.0-alpha.1](https://github.com/talos-systems/talos/compare/v0.8.0-alpha.0...v0.8.0-alpha.1) (2020-11-25)

### Chore

* add cloud image uploader (AWS AMIs for now)
* bump K8s to 1.19.4 in e2e scripts with CABPT version
* build arm64 images in CI
* remove maintenance service interface and use machine service

### Docs

* provide list of AMIs on AWS documentation page
* add 0.8 docs for the upcoming release
* ensure we configure nodes in guides
* ensure gcp docs have firewall and node info
* add qemu diagram and video walkthrough
* graduate v0.7 docs
* improve configuration reference documentation
* fix small typo in talosctl processes cast
* update asciinemas with talosctl
* add proxmox doc
* add live walkthroughs where applicable

### Feat

* support openstack platform
* update Kubernetes to v1.20.0-beta.2
* change UI component for disks selector
* support cluster expansion in the interactive installer
* implement apply configuration without reboot
* make GenerateConfiguration API reuse current node auth
* sync time before installer runs
* set interface MTU in DHCP mode even if DHCP is not successful
* print hint about using interative installer in mainenance mode
* add TUI based talos interactive installer
* support ipv6 routes
* return client config as the second value in GenerateConfiguration
* correctly merge talosconfig (don't ever overwrite)
* drop to maintenance mode in cloud platforms if userdata is missing
* read config from extra guestinfo key (vmware)
* update Go to 1.15.5
* add generate config gRPC API
* upgrade Kubernetes default version to 1.19.4
* add example command in maintenance, enforce cert fingerprint
* add storage API

### Fix

* bump blockdevice library for `mmcblk` part name fix
* ignore 'not found' errors when stopping/removing CRI pods
* return hostname from packet platform
* make fingerprint clearly optional in a boot hint
* ensure packet nics get all IPs
* use ghcr.io/talos-systems/kubelet
* bump timeout for config downloading on bare metal

### Refactor

* drop osd compatibility layer

### Test

* update integration test versions, clean up names

