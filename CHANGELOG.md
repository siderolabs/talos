
<a name="v0.8.2"></a>
## [v0.8.2](https://github.com/talos-systems/talos/compare/v0.8.1...v0.8.2) (2021-01-20)

### Feat

* allow fqdn to be used when registering k8s node

### Fix

* kill all processes and umount all disk on reboot/shutdown
* open blockdevices with exclusive flock for partitioning


<a name="v0.8.1"></a>
## [v0.8.1](https://github.com/talos-systems/talos/compare/v0.8.0...v0.8.1) (2021-01-12)

### Fix

* networkd updates for Packet, hostname detection, console kernel arg

### Release

* **v0.8.1:** prepare release


<a name="v0.8.0"></a>
## [v0.8.0](https://github.com/talos-systems/talos/compare/v0.8.0-beta.0...v0.8.0) (2020-12-23)

### Fix

* backport fixes from 0.9 after 0.8-beta.0

### Release

* **v0.8.0:** prepare release


<a name="v0.8.0-beta.0"></a>
## [v0.8.0-beta.0](https://github.com/talos-systems/talos/compare/v0.8.0-alpha.3...v0.8.0-beta.0) (2020-12-18)

### Chore

* lower MTU to 1450 for the tests in the CI
* build ISOs earlier to launch e2e-iso as soon as possible
* add drone pipeline to upload cloud images
* bump npm `ini` package for security vulnerability

### Docs

* add fallback to default page description if none is set on current page
* add a note for being careful about enabling debug flag

### Feat

* bump pkgs for kernel with HZ=250 on amd64
* bump Linux kernel to 5.10.1, add CONFIG_USB_ACM
* bump pkgs for kernel with CONFIG_USB_XHCI_PLATFORM

### Fix

* synchronize bootkube timeouts and various boot timeouts
* sync RTC in timed, sync time before fetching packet metadata
* don't overwrite PMBR
* bump blockdevice library for 2nd partitione entries copy fix
* properly define shorthand in `talosctl time` command
* take the first interface from the bond (packet)
* disable kmsg throttling for iso mode

### Refactor

* remove setup goroutine in etcd service

### Release

* **v0.8.0-beta.0:** prepare release

### Test

* add an extra 'node boot done' health check
* remove provision tests with Cilium CNI
* stabilize upgrade test by running health check several times


<a name="v0.8.0-alpha.3"></a>
## [v0.8.0-alpha.3](https://github.com/talos-systems/talos/compare/v0.8.0-alpha.2...v0.8.0-alpha.3) (2020-12-11)

### Chore

* update CONTRIBUTING.md
* limit unit-test run concurrency
* bump Go to 1.15.6
* bump dockerfile frontend version
* fix conform for releases

### Docs

* update Equinix Metal guide
* add architectural doc on the root file system layout
* add a note on caveats in container mode
* add storage doc
* add guide for custom CAs
* add docs for network connectivity
* improve SBC documentation

### Feat

* update kernel to 5.9.13, new KSPP requirements
* reset with system disk wipe spec
* add talosctl merge config command
* add talosctl config contexts
* update Kubernetes to 1.20.0
* implement "staged" (failsafe/backup) upgrades
* allow disabling NoSchedule taint on masters using TUI installer

### Fix

* remove kmsg ratelimiting on startup
* zero out partitions without filesystems on install
* make interactive installer work without endpoints provided

### Release

* **v0.8.0-alpha.3:** prepare release

### Test

* add ISO test
* add support for mounting ISO in talosctl cluster create
* bump Talos release version for upgrade test to 0.7.1
* bump defaults for provision tests resources


<a name="v0.8.0-alpha.2"></a>
## [v0.8.0-alpha.2](https://github.com/talos-systems/talos/compare/v0.8.0-alpha.1...v0.8.0-alpha.2) (2020-12-04)

### Chore

* publish Rock64 image
* enable thrice daily pipeline
* run integration test thrice daily
* output SBC images as compressed raw images
* build SBC images
* update module dependencies
* drop support for `docker load`
* fix metal image name
* use IMAGE_TAG instead of TAG for :latest pushes

### Docs

* fix typos
* add openstack docs
* ensure port for vbox and proxmox docs
* add console kernel arg to rpi_4 image generation
* add console kernel arg to libretech_all_h3_cc_h5 image generation

### Feat

* add support for the Pine64 Rock64
* add TUI for configuring network interfaces settings
* make GenerateConfiguration accept current time as a parameter
* introduce configpatcher package in machinery
* suggest fixed control plane endpoints in talosctl gen config
* update kubernetes to 1.20.0-rc.0
* allow boards to set kernel args
* add support for the Banana Pi M64
* stop including K8s version by default in `talosctl gen config`
* add support for the Raspberry Pi 4 Model B
* implement network interfaces list API
* bump package for kernel with CIFS support
* upgrade etcd to 3.4.14
* update Containerd and Linux
* add support for installing to SBCs
* add ability to choose CNI config

### Fix

* make default generate image arch dynamic based on arch
* stabilize serial console on RPi4, add video console
* make reset work again
* node taint doesn't contain value anymore
* defer resolving config context in client code
* remove value (change to empty) for `NoSchedule` taint
* prevent endless loop with DHCP requests in networkd
* skip `board` argument to the installer if it's not set
* use the dtb from kernel pkg for libretech_all_h3_cc_h5
* prevent crash in `talosctl config` commands
* update generated .ova manifest for raw disk size
* **security:** update Containerd to v1.4.3

### Release

* **v0.8.0-alpha.2:** prepare release


<a name="v0.8.0-alpha.1"></a>
## [v0.8.0-alpha.1](https://github.com/talos-systems/talos/compare/v0.8.0-alpha.0...v0.8.0-alpha.1) (2020-11-26)

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

### Release

* **v0.8.0-alpha.1:** prepare release

### Test

* update integration test versions, clean up names

