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
# Memory settings for the virtual machine.
memory:
    size: 4GiB # Memory allocated to the guest at boot.
    # Memory ballooning settings.
    ballooning:
        enabled: true # Attach a virtio-balloon device, letting the host reclaim memory the guest is not using.
# Firmware the virtual machine boots.
firmware:
    type: uefi # Firmware the guest boots.
    # Secure boot settings.
    secureBoot:
        enabled: true # Boot the guest with secure boot.
# Disks attached to the virtual machine.
disks:
    - name: system # Name of the disk, unique within the virtual machine.
      pool: pool1 # Name of the `StoragePoolConfig` document this disk's volume lives in.
      size: 20GiB # Size of the volume.
      bootOrder: 1 # Position of this disk in the guest's boot order, lowest first.
      # Where the volume's contents come from.
      provision:
        # Derive the volume from an image held in a content library.
        fromImage:
            library: images # Name of the `ContentLibraryConfig` document holding the image.
            file: talos-1.14.qcow2 # Name of the file within that library.
            mode: linked # How the volume is derived from the image.

            # # Integrity check of the library file, verified before the volume is provisioned.
            # digest: sha256:5f2bc19e8b4b5b4a8b5e9c0d1f2a3b4c5d6e7f8091a2b3c4d5e6f708192a3b4c
    - name: data # Name of the disk, unique within the virtual machine.
      pool: pool1 # Name of the `StoragePoolConfig` document this disk's volume lives in.
      size: 100GiB # Size of the volume.
      # Where the volume's contents come from.
      provision:
        # Create an empty volume, formatted per `format`.
        blank: {}
    - name: install # Name of the disk, unique within the virtual machine.
      pool: pool1 # Name of the `StoragePoolConfig` document this disk's volume lives in.
      type: cdrom # Kind of device the disk is presented as.
      bootOrder: 2 # Position of this disk in the guest's boot order, lowest first.
      # Where the volume's contents come from.
      provision:
        # Derive the volume from an image held in a content library.
        fromImage:
            library: images # Name of the `ContentLibraryConfig` document holding the image.
            file: ubuntu-24.04.iso # Name of the file within that library.

            # # Integrity check of the library file, verified before the volume is provisioned.
            # digest: sha256:5f2bc19e8b4b5b4a8b5e9c0d1f2a3b4c5d6e7f8091a2b3c4d5e6f708192a3b4c
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
        - name: net1 # Name of the interface, unique within the virtual machine.
          link: eth1 # Kernel name (or alias) of the host link the interface is attached to.
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




## cpu {#VirtualMachineConfig.cpu}

VirtualMachineCPU describes the processors presented to the guest.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`count` |uint32 |Number of virtual CPUs presented to the guest.<br><br>This is the total vCPU count, not a per-socket or per-core figure: how those vCPUs are<br>laid out into sockets, cores and threads is not configurable. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
count: 4
{{< /highlight >}}</details> | |






## memory {#VirtualMachineConfig.memory}

VirtualMachineMemory describes the memory presented to the guest.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`size` |ByteSize |Memory allocated to the guest at boot.<br><br>Size is specified in bytes, but can be expressed in human readable format, e.g. 4GiB. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
size: 4GiB
{{< /highlight >}}</details> | |
|`ballooning` |<a href="#VirtualMachineConfig.memory.ballooning">VirtualMachineBallooning</a> |Memory ballooning settings.<br><br>Optional; ballooning is disabled when this section is omitted.  | |




### ballooning {#VirtualMachineConfig.memory.ballooning}

VirtualMachineBallooning describes the virtio-balloon settings for a virtual machine.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`enabled` |bool |Attach a virtio-balloon device, letting the host reclaim memory the guest is not using.<br><br>Ballooning only shrinks the guest below `memory.size`; growing beyond it is memory<br>hot-add, which is a separate mechanism.<br><br>Optional; defaults to disabled.  | |








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
|`pool` |string |Name of the `StoragePoolConfig` document this disk's volume lives in.<br><br>The pool is declared separately and is not provisioned by this document. The reference<br>is checked for shape only: nothing resolves it against the rest of the machine<br>configuration yet. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
pool: pool1
{{< /highlight >}}</details> | |
|`size` |ByteSize |Size of the volume.<br><br>Size is specified in bytes, but can be expressed in human readable format, e.g. 20GiB.<br><br>Required for a `disk`, and not allowed on a `cdrom`, whose size is that of its image.  | |
|`format` |VirtualMachineDiskFormat |On-disk format of the volume.<br><br>This is not cosmetic: `provision.fromImage.mode: linked` requires `qcow2`, since backing<br>chains are a qcow2 feature, while `raw` is faster on block-backed pools.<br><br>Optional; defaults to `qcow2`. Not allowed on a `cdrom`, which is used as-is.  |`raw`<br />`qcow2`<br /> |
|`bus` |VirtualMachineDiskBus |Controller the disk is attached to.<br><br>`virtio` for anything modern; `sata` for guests without virtio drivers at install time.<br><br>Optional; defaults to `virtio` on a `disk` and to `sata` on a `cdrom`. A `cdrom` cannot<br>be attached to `virtio`, which presents no ejectable media.  |`virtio`<br />`scsi`<br />`sata`<br />`nvme`<br /> |
|`type` |VirtualMachineDiskType |Kind of device the disk is presented as.<br><br>A `cdrom` is read-only -- QEMU emulates no CD burner -- so its contents are required and<br>it has no size and no format of its own.<br><br>Optional; defaults to `disk`.  |`disk`<br />`cdrom`<br /> |
|`bootOrder` |uint32 |Position of this disk in the guest's boot order, lowest first.<br><br>Values must be unique across everything the virtual machine can boot from. Only disks<br>are bootable today, so that is only the disks; network interfaces will share this<br>namespace once they are configurable.<br><br>Optional; a disk without a boot order is not booted from.  | |
|`provision` |<a href="#VirtualMachineConfig.disks..provision">VirtualMachineDiskProvision</a> |Where the volume's contents come from.<br><br>Exactly one source must be set.  | |




### provision {#VirtualMachineConfig.disks..provision}

VirtualMachineDiskProvision describes where a disk's contents come from.

Exactly one source must be set.





| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`blank` |<a href="#VirtualMachineConfig.disks..provision.blank">VirtualMachineDiskBlank</a> |Create an empty volume, formatted per `format`.<br><br>Not allowed on a `cdrom`, which has no meaningful empty contents.  | |
|`fromImage` |<a href="#VirtualMachineConfig.disks..provision.fromImage">VirtualMachineDiskFromImage</a> |Derive the volume from an image held in a content library.  | |




#### blank {#VirtualMachineConfig.disks..provision.blank}

VirtualMachineDiskBlank provisions an empty volume.









#### fromImage {#VirtualMachineConfig.disks..provision.fromImage}

VirtualMachineDiskFromImage derives a volume from a content library image.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`library` |string |Name of the `ContentLibraryConfig` document holding the image. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
library: images
{{< /highlight >}}</details> | |
|`file` |string |Name of the file within that library. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
file: talos-1.14.qcow2
{{< /highlight >}}</details> | |
|`digest` |string |Integrity check of the library file, verified before the volume is provisioned.<br><br>Written as `<algorithm>:<hex>`, under either `sha256` or `sha512`.<br><br>Optional; the file is used as-is when this is unset. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
digest: sha256:5f2bc19e8b4b5b4a8b5e9c0d1f2a3b4c5d6e7f8091a2b3c4d5e6f708192a3b4c
{{< /highlight >}}</details> | |
|`mode` |VirtualMachineDiskImageMode |How the volume is derived from the image.<br><br>`copy` makes a full, independent copy. `linked` makes a thin qcow2 backed by the library<br>image: fast and space-cheap, but it pins that image for the lifetime of the disk, and it<br>requires `format: qcow2`.<br><br>Optional; defaults to `copy`.  |`copy`<br />`linked`<br /> |










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
|`link` |string |Kernel name (or alias) of the host link the interface is attached to.<br><br>The link must already exist on the host: it is attached to as is, and neither Talos nor<br>the hypervisor configures networking for it. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
link: eth0
{{< /highlight >}}</details> | |










