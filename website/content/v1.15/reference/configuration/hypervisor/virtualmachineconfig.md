---
description: |
    VirtualMachineConfig is a virtual machine configuration document.
    VirtualMachineConfig declares a virtual machine run by Talos.

    This document is the skeleton of the virtual machine: the rest of the machine's shape
    (firmware, disks, network interfaces, guest seeding, consoles) is added to it over time,
    and every addition is a new field rather than a change to an existing one.

    Status is reported via `VirtualMachineStatus`.
title: VirtualMachineConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: VirtualMachineConfig
name: vm1 # Name of the virtual machine.
powerState: running # Power state the virtual machine is driven towards.
# Processor settings for the virtual machine.
cpu:
    count: 4 # Number of virtual CPUs presented to the guest.
    limit: 3000m # Host CPU ceiling in millicores for the whole virtual machine, vCPUs and emulator threads
    # Optional guest CPU geometry and host CPU pinning.
    topology:
        sockets: 1 # Number of guest sockets. If any geometry dimension is set, all three must be
        cores: 2 # Number of guest cores per socket, not host CPU IDs.
        threads: 2 # Number of guest threads per core, not host CPU IDs.
        # Host CPUs the guest's threads are pinned to; independent of guest geometry and `cpu.limit`.
        pinning:
            # Per-vCPU pins.
            vcpus:
                - vcpu: 0 # Guest vCPU index, starting at 0 and below `cpu.count`.
                  cpus: "8" # Host CPUs the vCPU is pinned to, as a Linux CPU list, e.g. `8` or `9-10`.
                - vcpu: 1 # Guest vCPU index, starting at 0 and below `cpu.count`.
                  cpus: 9-10 # Host CPUs the vCPU is pinned to, as a Linux CPU list, e.g. `8` or `9-10`.
            emulator: 0-1 # Host CPUs the emulator threads (everything of the virtual machine that is not a vCPU)
# Memory settings for the virtual machine.
memory:
    size: 4GiB # Memory allocated to the guest at boot.
    # Memory ballooning settings.
    ballooning:
        enabled: true # Attach a virtio-balloon device, letting the host reclaim memory the guest is not using.
    # Host NUMA nodes the guest memory is placed on.
    numa:
        mode: strict # How guest memory is bound to the nodes.
        nodes: "1" # Host NUMA nodes the guest memory is placed on, as a Linux node list, e.g. `1` or `0-1`.
# Firmware the virtual machine boots.
firmware:
    type: uefi # Firmware the guest boots.
    # Secure boot settings.
    secureBoot:
        enabled: true # Boot the guest with secure boot.
# Disks attached to the virtual machine.
disks:
    - name: system # Name of the disk, unique within the virtual machine.
      pool: pool1 # Name of the `StoragePool` document this disk's volume lives in.
      size: 20GiB # Size of the volume.
      bootOrder: 1 # Position of this disk in the guest's boot order, lowest first.
      # Where the volume's contents come from.
      provision:
        # Create an empty volume, formatted per `format`.
        blank: {}
    - name: data # Name of the disk, unique within the virtual machine.
      pool: pool1 # Name of the `StoragePool` document this disk's volume lives in.
      size: 100GiB # Size of the volume.
      # Where the volume's contents come from.
      provision:
        # Create an empty volume, formatted per `format`.
        blank: {}
    - name: install # Name of the disk, unique within the virtual machine.
      type: cdrom # Kind of device the disk is presented as.
      bootOrder: 2 # Position of this disk in the guest's boot order, lowest first.
      # Where the volume's contents come from.
      provision:
        # Attach read-only CD-ROM media from a content library.
        fromImage:
            library: images # Name of the `ContentLibraryConfig` document holding the image.
            file: ubuntu-24.04.iso # Name of the file within that library.

            # # Integrity check of the library file, verified before the media is attached.
            # digest: sha256:5f2bc19e8b4b5b4a8b5e9c0d1f2a3b4c5d6e7f8091a2b3c4d5e6f708192a3b4c

      # # Name of the `StoragePool` document this disk's volume lives in.
      # pool: pool1
# Consoles attached to the virtual machine.
console:
    # Serial console settings.
    serial:
        enabled: true # Attach a serial console.
    # VNC console settings.
    vnc:
        enabled: true # Attach a VNC console.
# Networking settings for the virtual machine.
networking:
    # Network interfaces presented to the guest.
    interfaces:
        - name: net0 # Name of the interface, unique within the virtual machine.
          link: eth0 # Kernel name (or alias) of the host link the interface is attached to.

          # # Hardware (MAC) address presented to the guest.
          # hardwareAddr: 52:54:00:12:34:56
        - name: net1 # Name of the interface, unique within the virtual machine.
          link: eth1 # Kernel name (or alias) of the host link the interface is attached to.

          # # Hardware (MAC) address presented to the guest.
          # hardwareAddr: 52:54:00:12:34:56
# Settings which apply inside the guest.
guest:
    # Seed handed to the guest on first boot.
    cloudInit:
        library: targetlibrary # Name of the content library where Talos stores the generated NoCloud ISO.
        metaData: | # Contents of the seed's `meta-data` file, carrying the guest's identity.
            instance-id: vm1-001
            local-hostname: vm1
        userData: | # Contents of the seed's `user-data` file, carrying what the operator wants done.
            #cloud-config
            users:
              - name: op
                ssh_authorized_keys:
                  - ssh-ed25519 AAAAC3NzaC1lZDI1NTE5...
    # qemu-guest-agent settings.
    agent:
        enabled: true # Attach the qemu-guest-agent virtio-serial channel to the domain.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of the virtual machine.<br><br>Must be between 1 and 63 characters long, and can only contain ASCII letters,<br>digits and hyphens. It is the ID used to address the virtual machine over the API.  | |
|`powerState` |PowerState |Power state the virtual machine is driven towards.  |`running`<br />`stopped`<br />`suspended`<br /> |
|`cpu` |<a href="#VirtualMachineConfig.cpu">VirtualMachineCPU</a> |Processor settings for the virtual machine.  | |
|`memory` |<a href="#VirtualMachineConfig.memory">VirtualMachineMemory</a> |Memory settings for the virtual machine.  | |
|`firmware` |<a href="#VirtualMachineConfig.firmware">VirtualMachineFirmware</a> |Firmware the virtual machine boots.  | |
|`disks` |<a href="#VirtualMachineConfig.disks.">[]VirtualMachineDisk</a> |Disks attached to the virtual machine.<br><br>Removing a disk detaches it from the virtual machine; the volume backing it stays in<br>its storage pool and is deleted separately.<br><br>A configuration patch merges into this list by disk name: a patch entry naming an<br>existing disk updates that disk, and any other entry is appended. Removing a disk<br>requires supplying the document in full.  | |
|`console` |<a href="#VirtualMachineConfig.console">VirtualMachineConsole</a> |Consoles attached to the virtual machine.<br><br>Optional; omitting it leaves both consoles detached.  | |
|`networking` |<a href="#VirtualMachineConfig.networking">VirtualMachineNetworking</a> |Networking settings for the virtual machine.<br><br>Optional; a virtual machine with no interfaces has no network connectivity at all.  | |
|`guest` |<a href="#VirtualMachineConfig.guest">VirtualMachineGuest</a> |Settings which apply inside the guest.<br><br>Optional; omitting it leaves the guest to boot its image unmodified.  | |




## cpu {#VirtualMachineConfig.cpu}

VirtualMachineCPU describes the processors presented to the guest and the host time they may consume.

There is deliberately no matching memory ceiling. Guest memory is already fixed by `memory.size`,
and libvirt advises against a QEMU memory hard limit: the emulator's own footprint over guest RAM
is not predictable, and a limit guessed too low has the kernel kill the virtual machine.





| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`count` |uint32 |Number of virtual CPUs presented to the guest.<br><br>This is the total vCPU count, not a per-socket or per-core figure: how those vCPUs are<br>laid out into sockets, cores and threads can be configured with `topology`. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
count: 4
{{< /highlight >}}</details> | |
|`limit` |string |Host CPU ceiling in millicores for the whole virtual machine, vCPUs and emulator threads<br>together, mapped onto the domain's global CFS quota.<br><br>`1000m` is one host core. The ceiling is independent of `count`: a guest with four<br>vCPUs and a `2000m` ceiling sees four processors but is scheduled for at most two cores<br>of host time.<br><br>Optional; omitting it leaves the virtual machine bounded only by its vCPU count. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
limit: 3000m
{{< /highlight >}}</details> | |
|`topology` |<a href="#VirtualMachineConfig.cpu.topology">VirtualMachineCPUTopology</a> |Optional guest CPU geometry and host CPU pinning.<br><br>Geometry counts describe the guest, not host CPU IDs. When any dimension is set,<br>all three must be positive and their product must equal `count`.<br>Pinning may be configured without guest geometry.  | |




### topology {#VirtualMachineConfig.cpu.topology}

VirtualMachineCPUTopology describes guest CPU geometry and independent host CPU pinning.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`sockets` |uint32 |Number of guest sockets. If any geometry dimension is set, all three must be<br>positive and sockets * cores * threads must equal `cpu.count`.  | |
|`cores` |uint32 |Number of guest cores per socket, not host CPU IDs.  | |
|`threads` |uint32 |Number of guest threads per core, not host CPU IDs.  | |
|`pinning` |<a href="#VirtualMachineConfig.cpu.topology.pinning">VirtualMachineCPUPinning</a> |Host CPUs the guest's threads are pinned to; independent of guest geometry and `cpu.limit`.<br>Optional; omitting it leaves the virtual machine schedulable on any host CPU.  | |




#### pinning {#VirtualMachineConfig.cpu.topology.pinning}

VirtualMachineCPUPinning describes which host CPUs the guest's threads are pinned to.

Host CPU IDs are the kernel's logical CPU numbers, SMT threads included. Whether a named CPU
exists and is available is only known on the host, when the virtual machine starts.





| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`vcpus` |<a href="#VirtualMachineConfig.cpu.topology.pinning.vcpus.">[]VirtualMachineVCPUPin</a> |Per-vCPU pins.<br><br>A vCPU left out of this list is scheduled on any host CPU.<br><br>A configuration patch replaces this list as a whole rather than appending to it.  | |
|`emulator` |string |Host CPUs the emulator threads (everything of the virtual machine that is not a vCPU)<br>are pinned to, as a Linux CPU list, e.g. `0-1,4`.<br><br>Optional; omitting it leaves the emulator threads unpinned. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
emulator: 0-1
{{< /highlight >}}</details> | |




##### vcpus[] {#VirtualMachineConfig.cpu.topology.pinning.vcpus.}

VirtualMachineVCPUPin pins one guest vCPU to a set of host CPUs.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`vcpu` |uint32 |Guest vCPU index, starting at 0 and below `cpu.count`. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
vcpu: 0
{{< /highlight >}}</details> | |
|`cpus` |string |Host CPUs the vCPU is pinned to, as a Linux CPU list, e.g. `8` or `9-10`. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
cpus: 9-10
{{< /highlight >}}</details> | |












## memory {#VirtualMachineConfig.memory}

VirtualMachineMemory describes the memory presented to the guest.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`size` |ByteSize |Memory allocated to the guest at boot.<br><br>Size is specified in bytes, but can be expressed in human readable format, e.g. 4GiB. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
size: 4GiB
{{< /highlight >}}</details> | |
|`ballooning` |<a href="#VirtualMachineConfig.memory.ballooning">VirtualMachineBallooning</a> |Memory ballooning settings.<br><br>Optional; ballooning is disabled when this section is omitted.  | |
|`numa` |<a href="#VirtualMachineConfig.memory.numa">VirtualMachineNUMA</a> |Host NUMA nodes the guest memory is placed on.<br><br>Optional; omitting it leaves placement to the host.  | |




### ballooning {#VirtualMachineConfig.memory.ballooning}

VirtualMachineBallooning describes the virtio-balloon settings for a virtual machine.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`enabled` |bool |Attach a virtio-balloon device, letting the host reclaim memory the guest is not using.<br><br>Ballooning only shrinks the guest below `memory.size`; growing beyond it is memory<br>hot-add, which is a separate mechanism.<br><br>Optional; defaults to disabled.  | |






### numa {#VirtualMachineConfig.memory.numa}

VirtualMachineNUMA describes where the guest memory is placed on the host.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`mode` |VirtualMachineNUMAMode |How guest memory is bound to the nodes.<br><br>`strict` fails allocations that cannot be served from `nodes`; `preferred` falls back<br>to other nodes; `interleave` spreads pages across `nodes` round-robin.<br><br>Optional; defaults to `strict`.  |`strict`<br />`preferred`<br />`interleave`<br /> |
|`nodes` |string |Host NUMA nodes the guest memory is placed on, as a Linux node list, e.g. `1` or `0-1`.<br><br>Whether a named node exists is only known on the host, when the virtual machine starts. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
nodes: "1"
{{< /highlight >}}</details> | |








## firmware {#VirtualMachineConfig.firmware}

VirtualMachineFirmware describes the firmware a virtual machine boots.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`type` |VirtualMachineFirmwareType |Firmware the guest boots.<br><br>Required, and unchangeable for the life of the guest: a guest installed under one<br>firmware will not boot under the other. `uefi` is the only option on arm64, and the only<br>one secure boot can be used with.  |`uefi`<br />`bios`<br /> |
|`secureBoot` |<a href="#VirtualMachineConfig.firmware.secureBoot">VirtualMachineFirmwareSecureBoot</a> |Secure boot settings.<br><br>Optional; secure boot is disabled when this section is omitted.  | |




### secureBoot {#VirtualMachineConfig.firmware.secureBoot}

VirtualMachineFirmwareSecureBoot describes the secure boot settings of a virtual machine.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`enabled` |bool |Boot the guest with secure boot.<br><br>Requires `type: uefi`. Talos keeps a per-virtual-machine variable store alongside the<br>rest of the machine's identity, seeded from a signed firmware template and preserved for<br>the life of the guest, as it is where the enrolled keys live.<br><br>Optional; defaults to disabled.  | |








## disks[] {#VirtualMachineConfig.disks.}

VirtualMachineDisk describes a single disk attached to a virtual machine.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of the disk, unique within the virtual machine.<br><br>Must be between 1 and 63 characters long, and can only contain ASCII letters,<br>digits and hyphens. It names the volume created in the storage pool.  | |
|`pool` |string |Name of the `StoragePool` document this disk's volume lives in.<br><br>The pool is declared separately and is not provisioned by this document, but it must be<br>declared: a disk naming a pool no `StoragePool` document declares is a configuration<br>error.<br><br>The volume is named after this virtual machine and this disk, so a volume of that name<br>already in the pool is adopted with its existing contents. Removing the disk from the<br>configuration never deletes the volume, so re-declaring the same virtual machine and disk<br>names in the same pool reattaches the same data.<br><br>Required for a `disk`, and not allowed on a `cdrom`, whose image is attached in place<br>from its content library and never lands in a pool. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
pool: pool1
{{< /highlight >}}</details> | |
|`size` |ByteSize |Size of the volume.<br><br>Size is specified in bytes, but can be expressed in human readable format, e.g. 20GiB.<br><br>Required for a `disk`, and not allowed on a `cdrom`, whose size is that of its image.  | |
|`format` |VirtualMachineDiskFormat |On-disk format of a blank volume.<br><br>Choose `raw` or `qcow2` for a writable disk provisioned with `blank`. The format is not<br>inferred from a content library image. Writable image-derived disks are not supported.<br><br>Optional; defaults to `qcow2`. Not allowed on a `cdrom`, which is used as-is.  |`raw`<br />`qcow2`<br /> |
|`bus` |VirtualMachineDiskBus |Controller the disk is attached to.<br><br>`virtio` for anything modern; `sata` for guests without virtio drivers at install time.<br><br>Optional; defaults to `virtio` on a `disk` and to `sata` on a `cdrom`. A `cdrom` cannot<br>be attached to `virtio`, which presents no ejectable media.  |`virtio`<br />`scsi`<br />`sata`<br /> |
|`type` |VirtualMachineDiskType |Kind of device the disk is presented as.<br><br>A `cdrom` is read-only -- QEMU emulates no CD burner -- so its contents are required and<br>it has no size and no format of its own.<br><br>Optional; defaults to `disk`.  |`disk`<br />`cdrom`<br /> |
|`bootOrder` |uint32 |Position of this disk in the guest's boot order, lowest first.<br><br>Values must be unique across everything the virtual machine can boot from. Only disks<br>are bootable today, so that is only the disks; network interfaces will share this<br>namespace once they are configurable.<br><br>Optional; a disk without a boot order is not booted from.  | |
|`provision` |<a href="#VirtualMachineConfig.disks..provision">VirtualMachineDiskProvision</a> |Where the volume's contents come from.<br><br>Exactly one source must be set.  | |




### provision {#VirtualMachineConfig.disks..provision}

VirtualMachineDiskProvision describes where a disk's contents come from.

Exactly one source must be set.





| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`blank` |<a href="#VirtualMachineConfig.disks..provision.blank">VirtualMachineDiskBlank</a> |Create an empty volume, formatted per `format`.<br><br>Not allowed on a `cdrom`, which has no meaningful empty contents.  | |
|`fromImage` |<a href="#VirtualMachineConfig.disks..provision.fromImage">VirtualMachineDiskFromImage</a> |Attach read-only CD-ROM media from a content library.<br><br>Set `type: cdrom` and omit `pool`, `size`, `format`, and `mode`; the library file is<br>attached in place, not copied into a volume. Writable image-derived disks (copy or<br>linked) are not supported. Use `provision.blank` for a writable disk and install from<br>CD-ROM media.  | |




#### blank {#VirtualMachineConfig.disks..provision.blank}

VirtualMachineDiskBlank provisions an empty volume.









#### fromImage {#VirtualMachineConfig.disks..provision.fromImage}

VirtualMachineDiskFromImage references media in a content library.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`library` |string |Name of the `ContentLibraryConfig` document holding the image. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
library: images
{{< /highlight >}}</details> | |
|`file` |string |Name of the file within that library. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
file: ubuntu-24.04.iso
{{< /highlight >}}</details> | |
|`digest` |string |Integrity check of the library file, verified before the media is attached.<br><br>Written as `<algorithm>:<hex>`, under either `sha256` or `sha512`.<br><br>Optional; the file is used as-is when this is unset. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
digest: sha256:5f2bc19e8b4b5b4a8b5e9c0d1f2a3b4c5d6e7f8091a2b3c4d5e6f708192a3b4c
{{< /highlight >}}</details> | |
|`mode` |VirtualMachineDiskImageMode |Mode for deriving a writable disk from an image (not supported yet).<br><br>Neither `copy` nor `linked` currently provisions a writable image-derived disk. For a<br>writable disk, use `provision.blank` instead; for image media, use a read-only `cdrom`.<br><br>Omit this field on a `cdrom`: even an explicit `copy` is rejected because its read-only<br>medium is attached in place. An omitted mode defaults to `copy` for a disk, but that<br>writable image-derived configuration is not supported.  |`copy`<br />`linked`<br /> |










## console {#VirtualMachineConfig.console}

VirtualMachineConsole describes the consoles attached to a virtual machine.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`serial` |<a href="#VirtualMachineConfig.console.serial">VirtualMachineSerial</a> |Serial console settings.  | |
|`vnc` |<a href="#VirtualMachineConfig.console.vnc">VirtualMachineVNC</a> |VNC console settings.  | |




### serial {#VirtualMachineConfig.console.serial}

VirtualMachineSerial describes the serial console of a virtual machine.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`enabled` |bool |Attach a serial console.<br><br>Optional; defaults to disabled.  | |






### vnc {#VirtualMachineConfig.console.vnc}

VirtualMachineVNC describes the VNC console of a virtual machine.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`enabled` |bool |Attach a VNC console.<br><br>Optional; defaults to disabled.  | |








## networking {#VirtualMachineConfig.networking}

VirtualMachineNetworking describes the networking of a virtual machine.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`interfaces` |<a href="#VirtualMachineConfig.networking.interfaces.">[]VirtualMachineInterface</a> |Network interfaces presented to the guest.<br><br>Removing an interface from this list detaches it from the virtual machine.<br><br>A configuration patch replaces this list as a whole rather than appending to it.  | |




### interfaces[] {#VirtualMachineConfig.networking.interfaces.}

VirtualMachineInterface describes a single network interface of a virtual machine.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of the interface, unique within the virtual machine.<br><br>Must be between 1 and 63 characters long, and can only contain ASCII letters,<br>digits and hyphens. This is how the interface is addressed over the API; it is not the<br>device name inside the guest, which the guest kernel chooses for itself. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
name: net0
{{< /highlight >}}</details> | |
|`link` |string |Kernel name (or alias) of the host link the interface is attached to.<br><br>The link must already exist on the host and be an Ethernet link, e.g. a physical<br>interface, a bond, or a VLAN. It is attached to as is. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
link: eth0
{{< /highlight >}}</details> | |
|`hardwareAddr` |HardwareAddr |Hardware (MAC) address presented to the guest.<br><br>Defaults to an address derived from the virtual machine and interface names, which is<br>stable for as long as both keep their names. Set it to pin the address a DHCP server<br>reserves against, or to keep one across a rename.<br><br>It must be a unicast address, and is not allowed to be all zeroes. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
hardwareAddr: 52:54:00:12:34:56
{{< /highlight >}}</details> | |








## guest {#VirtualMachineConfig.guest}

VirtualMachineGuest describes the settings which apply inside the guest.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`cloudInit` |<a href="#VirtualMachineConfig.guest.cloudInit">VirtualMachineCloudInit</a> |Seed handed to the guest on first boot.  | |
|`agent` |<a href="#VirtualMachineConfig.guest.agent">VirtualMachineAgent</a> |qemu-guest-agent settings.<br><br>Optional; omitting this section is equivalent to the default `{enabled: true}`.<br>Set `agent.enabled: false` to omit the channel entirely.  | |




### cloudInit {#VirtualMachineConfig.guest.cloudInit}

VirtualMachineCloudInit describes the NoCloud seed handed to the guest.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`library` |string |Name of the content library where Talos stores the generated NoCloud ISO.<br><br>Must name a content library with a nonempty identifier of at most 63 ASCII letters,<br>digits or hyphens. The library is declared separately by a `ContentLibraryConfig`. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
library: targetlibrary
{{< /highlight >}}</details> | |
|`metaData` |string |Contents of the seed's `meta-data` file, carrying the guest's identity.<br><br>`instance-id` is what decides whether a boot is a reboot or a new instance. An unchanged<br>id means edits to `userData` are inert; a changed id re-runs provisioning, which<br>regenerates the SSH host keys in most images.<br><br>Must be valid YAML. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
metaData: |
    instance-id: vm1-001
    local-hostname: vm1
{{< /highlight >}}</details> | |
|`userData` |string |Contents of the seed's `user-data` file, carrying what the operator wants done.<br><br>For a distro image this is a cloud-init document, usually starting with `#cloud-config`,<br>though a script or a MIME archive is equally valid -- it is not parsed here. For a Talos<br>guest this is the guest's own machine configuration.<br><br>Carries SSH keys, passwords and tokens in practice, and is redacted from the<br>configuration as read back over the API.  | |
|`networkConfig` |string |Contents of the seed's `network-config` file, carrying the guest's network settings.<br><br>Needed by guests which cannot configure themselves over DHCP. Unset leaves the guest to<br>its own defaults.<br><br>Must be valid YAML.  | |






### agent {#VirtualMachineConfig.guest.agent}

VirtualMachineAgent describes the qemu-guest-agent settings for a virtual machine.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`enabled` |bool |Attach the qemu-guest-agent virtio-serial channel to the domain.<br><br>The host does not use the channel yet; this field is the plumbing that lets future<br>features (such as reporting guest addresses or requesting a graceful shutdown) talk to<br>a qemu-guest-agent process inside the guest.<br><br>Optional; defaults to enabled. Set to `false` to omit the channel entirely.  | |










