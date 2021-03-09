<a name="v0.9.0-beta.0"></a>
## [v0.9.0-beta.0](https://github.com/talos-systems/talos/compare/v0.9.0-alpha.5...v0.9.0-beta.0) (2021-03-09)

### Chore

* bump release scope to v0.9
* bump dependencies via dependabot
* update tools/pkgs/extras tags
* fix //nolint directives
* expire objects in CI S3 bucket
* fix upgrade tests by bumping 0.9 to alpha.5

### Docs

* document controller runtime, resources and talosctl get

### Feat

* update containerd to version 1.4.4

### Fix

* do not print out help string if the parameters are correct
* mkdir source of the extra mounts for the kubelet
* properly propagate nameservers to provisioned docker clusters
* ignore connection refused errors when updating/converting cp
* align partition start to the physical sector size
* chmod etcd PKI path to fix virtual IP for upgrades with persistence

<a name="v0.9.0-alpha.5"></a>
## [v0.9.0-alpha.5](https://github.com/talos-systems/talos/compare/v0.9.0-alpha.4...v0.9.0-alpha.5) (2021-03-03)

### Chore

* bump Go module dependencies
* properly propagate context object in the controller

### Feat

* bypass lock if ACPI reboot/shutdown issued
* add `--on-reboot` flag to talosctl edit/patch machineConfig
* support JSON output in `talosctl get`, event types
* rename namespaces, resources, types etc

### Release

* **v0.9.0-alpha.5:** prepare release

<a name="v0.9.0-alpha.4"></a>
## [v0.9.0-alpha.4](https://github.com/talos-systems/talos/compare/v0.9.0-alpha.3...v0.9.0-alpha.4) (2021-03-02)

### Chore

* update provision/upgrade tests to 0.9.0-alpha.3

### Docs

* bump v0.8 release version in the SBCs guides
* add disk encryption guide

### Feat

* update linux kernel to 5.10.19

### Fix

* ignore 'ENOENT' (no such file directory) on mount
* move etcd to `cri` containerd runner

### Release

* **v0.9.0-alpha.4:** prepare release

<a name="v0.9.0-alpha.3"></a>
## [v0.9.0-alpha.3](https://github.com/talos-systems/talos/compare/v0.9.0-alpha.2...v0.9.0-alpha.3) (2021-03-01)

### Chore

* bump dependencies via dependabot
* build both Darwin and Linux versions of talosctl
* bump dependencies via dependabot
* switch CI to stop embedding local registry into the builds

### Docs

* update AMI images for 0.8.4

### Feat

* implement etcd remove-member cli command
* update etcd to 3.4.15
* talosctl: allow v-prefixed k8s versions
* implement simple layer 2 shared IP for CP
* implement talosctl edit and patch config commands
* bump etcd client library to 3.5.0-alpha.0

### Fix

* update in-cluster kubeconfig validity to match other certs
* add ApplyDynamicConfig call in the apply-config --immediate mode
* set hdmi_safe=1 on Raspberry Pi for maximum HDMI compatibility
* show stopped/exited containers via CRI inspector
* make ApplyDynamicConfig idempotent
* improve the drain function
* correctly set service state in the resource
* update the layout of the Disks API to match proxying requirements
* stop and clean up installer container  correctly
* sanitize volume name better in static pod extra volumes

### Refactor

* add context to the networkd
* split WithNetworkConfig into sub-options

### Release

* **v0.9.0-alpha.3:** prepare release

### Test

* add integration test with Canal CNI and reset API
* upgrade master to master tests

<a name="v0.9.0-alpha.2"></a>
## [v0.9.0-alpha.2](https://github.com/talos-systems/talos/compare/v0.9.0-alpha.1...v0.9.0-alpha.2) (2021-02-20)

### Chore

* add default cron pipeline to the list of pipelines
* run default pipeline as part of the `cron` pipeline

### Docs

* add link to GitHub Discussions as a support forum

### Feat

* u-boot 2021.01, ca-certificates update, Linux file ACLs
* support control plane upgrades with Talos managed control plane
* add support for extra volume mounts for control plane pods
* add a warning to boot log if running self-hosted control plane
* add an option to disable kube-proxy manifest
* update Kubernetes to 1.20.4
* add state encryption support

### Fix

* redirect warnings in manifest apply k8s client
* handle case when kubelet serving certificates are issued
* correctly escape extra args in kube-proxy manifest
* skip empty manifest YAML sub-documents

### Refactor

* split kubernetes/etcd resource generation into subresources

### Release

* **v0.9.0-alpha.2:** prepare release

### Test

* enable disk encryption key rotation test
* update integration tests to use wrapped client for etcd APIs

<a name="v0.9.0-alpha.1"></a>
## [v0.9.0-alpha.1](https://github.com/talos-systems/talos/compare/v0.9.0-alpha.0...v0.9.0-alpha.1) (2021-02-18)

### Chore

* update artifacts bucket name in Drone
* rework Drone pipelines
* update dependencies via dependabot
* **ci:** fix schedules in Drone pipelines
* **ci:** update gcp templates

### Docs

* update AMI list for 0.8.2
* fix typos

### Feat

* add a tool and package to convert self-hosted CP to static pods
* implement ephemeral partition encryption
* add resource watch API + CLI
* rename apply-config --no-reboot to --on-reboot
* skip filesystem for state and ephemeral partitions in the installer
* stop all pods before unmounting ephemeral partition
* bump Go to 1.15.8
* support version contract for Talos config generation
* update Linux to 5.10.14
* add an option to force upgrade without checks
* upgrade CoreDNS to 1.8.0
* implement IPv6 DHCP client in networkd

### Fix

* correctly unwrap responses for etcd commands
* drop cri dependency on etcd
* move versions to annotations in control plane static pods
* find master node IPs correctly in health checks
* add 3 seconds grub boot timeout
* don't use filename from URL when downloading manifest
* pass attributes when adding routes
* correct response structure for GenerateConfig API
* correctly extract wrapped error messages
* prevent crash in machined on apid service stop
* wait for time sync before generating Kubernetes certificates
* set proper hostname on docker nodes
* mount kubelet secrets from system instead of ephemeral
* allow loading of empty config files
* prefer configured nameservers, fix DHCP6 in container
* refresh control plane endpoints on worker apids on schedule
* update DHCP client to use Request-Ack sequence after an Offer

### Refactor

* extract go-cmd into a separate library

### Release

* **v0.9.0-alpha.1:** prepare release

### Test

* trigger e2e on thrice daily
* update aws templates
* add support for IPv6 in talosctl cluster create

<a name="v0.9.0-alpha.0"></a>
## [v0.9.0-alpha.0](https://github.com/talos-systems/talos/compare/v0.8.1...v0.9.0-alpha.0) (2021-02-01)

### Chore

* bump dependencies (via dependabot)
* fix import path for fsnotify
* add dependabot config
* enable virtio-balloon and monitor in QEMU provisioner
* update protobuf, grpc-go, prototool
* update upgrade test version used

### Docs

* update components.md
* add v0.9 docs
* add modes to validate command
* document omitting DiskPartition size
* update references to 0.8.0, add 0.8.0 AWS AMIs
* fix latest docs
* set latest docs to v0.8
* provide AMIs for 0.8.0-beta.0
* fix SBC docs to point to beta.0 instead of beta.1
* update Talos release for SBCs

### Feat

* move to ECDSA keys for all Kubernetes/etcd certs and keys
* update kernel
* mount hugetlbfs
* allow fqdn to be used when registering k8s node
* copy cryptsetup executable from pkgs
* use multi-arch images for k8s and Flannel CNI
* replace bootkube with Talos-managed control plane
* implement resource API in Talos
* update Linux to 5.10.7, musl-libc to 1.2.2
* update Kubernetes to 1.20.2
* support Wireguard networking
* bump pkgs for kernel with CONFIG_IPV6_MULTIPLE_TABLES
* support type filter in list API and CLI
* add commands to manage/query etcd cluster
* support disk image in talosctl cluster create
* update Kubernetes to 1.20.1

### Fix

* use hugetlbfs instead of none
* use grpc load-balancing when connecting to trustd
* lower memory usage a bit by disabling memory profiling
* don't probe disks in container mode
* prefix rendered Talos-owned static pod manifests
* bump timeout for worker apid waiting for kubelet client config
* kill all processes and umount all disk on reboot/shutdown
* open blockdevices with exclusive flock for partitioning
* list command unlimited recursion default behavior
* pick first interface valid hostname (vs. last one)
* allow 'console' argument in kernel args to be always overridden
* bring up bonded interfaces correctly on packet
* checkpoint controller-manager and scheduler
* correctly transport gRPC errors from apid
* use SetAll instead of AppendAll when building kernel args
* add more dependencies for bootstrap services
* pass disk image flags to e2e-qemu cluster create command
* ignore pods spun up from checkpoints in health checks
* leave etcd for staged upgrades
* ignore errors on stopping/removing pod sandboxes
* use the correct console on Banana Pi M64
* don't run LabelNodeAsMaster in two sequences

### Refactor

* update go-blockdevice and restructure disk interaction code
* define default kernel flags in machinery instead of procfs

### Release

* **v0.9.0-alpha.0:** prepare release

### Test

* clear connection refused errors after reset
* skip etcd tests on non-HA clusters
