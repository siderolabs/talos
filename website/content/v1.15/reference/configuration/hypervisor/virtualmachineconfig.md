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
# Processor settings for the virtual machine.
cpu:
    count: 4 # Number of virtual CPUs presented to the guest.
# Memory settings for the virtual machine.
memory:
    size: 4GiB # Memory allocated to the guest at boot.
    # Memory ballooning settings.
    ballooning:
        enabled: true # Attach a virtio-balloon device, letting the host reclaim memory the guest is not using.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of the virtual machine.<br><br>Must be between 1 and 63 characters long, and can only contain ASCII letters,<br>digits and hyphens. It is the ID used to address the virtual machine over the API.  | |
|`cpu` |<a href="#VirtualMachineConfig.cpu">VirtualMachineCPU</a> |Processor settings for the virtual machine.  | |
|`memory` |<a href="#VirtualMachineConfig.memory">VirtualMachineMemory</a> |Memory settings for the virtual machine.  | |




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










