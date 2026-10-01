---
description: |
    CPUPartitionConfig is a CPU partition config document.
    CPUPartitionConfig bounds the host CPUs each fixed Talos workload root may run on.

    The roots are the cgroups Talos already creates (`init`, `system`, `podruntime`,
    `taloscontainers`, `virtualMachines`) plus `kubepods`, the cgroup the kubelet
    creates for pods. A root left out of the document keeps running on every host CPU;
    a root listed in the document must name at least one CPU. Roots may overlap each other.

    The virtual machine root may additionally be divided into named slices which
    `VirtualMachineConfig` selects with `cpu.slice`. Virtual machines which select no
    slice run on the remainder of the virtual machine root, never on a named slice.

    An exclusive slice must not overlap any other root, and every other root must then be
    explicitly bounded. At most one virtual machine may select an exclusive slice.

    When `kubepods` is bounded, Talos owns the kubelet's `reservedSystemCPUs`.

    Changing the CPUs of a root or slice is applied live to running virtual machines
    when it can be done without ever letting an exclusive slice share a CPU with another
    workload and without moving a virtual machine between slices. Transitions which
    cannot be applied that way (introducing or removing slices while virtual machines
    run in the virtual machine root, changing a virtual machine's `cpu.slice`,
    swapping the CPUs of two occupied slices, shrinking a slice under a pinned virtual
    machine) are rejected and reported with the affected virtual machines; Talos never
    pauses, stops or restarts a virtual machine to apply a CPU policy change. To perform
    such a transition, set the affected virtual machines to `powerState: stopped`, wait
    until `CPUPartitionStatus` no longer lists them as blocking (Talos verifies the domain
    is removed and the slice cgroup is empty; a stopped `VirtualMachineStatus` alone is not
    release), apply the change, then set them back to `running`.

    If a bounded CPU goes offline or a managed cgroup mask is changed outside Talos, the
    loss is reported in `CPUPartitionStatus` and new virtual machine starts are refused;
    virtual machines already running are left alone, so their isolation may no longer hold
    until the boundary is restored. Talos never stops or restarts a virtual machine to
    react to such a loss.

    Removing the document restores every root it bounded. A root which had tasks before
    the policy and inherited its CPUs cannot be set back to inheriting while it has tasks
    (the kernel refuses an empty cpuset), so it is given its parent's full CPU set, which
    is the same set it ran on before; empty slices are removed.

    In container mode the document is validated but has no effect.
title: CPUPartitionConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: CPUPartitionConfig
# Host CPUs for machined and the early boot services.
init:
    cpus: 0-1 # Host CPUs the root may run on, as a Linux CPU list, e.g. `0-1` or `0,2-3`.
# Host CPUs for Talos system services (apid, trustd, udevd, ...).
system:
    cpus: 0-1 # Host CPUs the root may run on, as a Linux CPU list, e.g. `0-1` or `0,2-3`.
# Host CPUs for the Kubernetes runtime components (containerd, the kubelet, etcd).
podruntime:
    cpus: 0-1 # Host CPUs the root may run on, as a Linux CPU list, e.g. `0-1` or `0,2-3`.
# Host CPUs for Kubernetes pods.
kubepods:
    cpus: 2-3 # Host CPUs the root may run on, as a Linux CPU list, e.g. `0-1` or `0,2-3`.
# Host CPUs for containers declared via `ContainerConfig`.
taloscontainers:
    cpus: 0-1 # Host CPUs the root may run on, as a Linux CPU list, e.g. `0-1` or `0,2-3`.
# Host CPUs for virtual machines declared via `VirtualMachineConfig`, optionally
virtualMachines:
    cpus: 4-7 # Host CPUs the virtual machine root may run on, as a Linux CPU list.
    # Named, pairwise disjoint subsets of `cpus` a `VirtualMachineConfig` selects with
    slices:
        - name: database # Name of the slice, unique within the document.
          cpus: 6-7 # Host CPUs of the slice, as a Linux CPU list. Must be a subset of the virtual
          exclusive: true # Reserve the slice for a single virtual machine.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`init` |<a href="#CPUPartitionConfig.init">CPUPartitionRoot</a> |Host CPUs for machined and the early boot services.<br><br>Optional; omitting it leaves the root unrestricted.  | |
|`system` |<a href="#CPUPartitionConfig.system">CPUPartitionRoot</a> |Host CPUs for Talos system services (apid, trustd, udevd, ...).<br><br>Optional; omitting it leaves the root unrestricted.  | |
|`podruntime` |<a href="#CPUPartitionConfig.podruntime">CPUPartitionRoot</a> |Host CPUs for the Kubernetes runtime components (containerd, the kubelet, etcd).<br><br>Optional; omitting it leaves the root unrestricted.  | |
|`kubepods` |<a href="#CPUPartitionConfig.kubepods">CPUPartitionRoot</a> |Host CPUs for Kubernetes pods.<br><br>Bounding this root makes Talos set the kubelet's `reservedSystemCPUs` to the<br>complement of it; the kubelet configuration must not set that field itself.<br><br>Optional; omitting it leaves the root unrestricted.  | |
|`taloscontainers` |<a href="#CPUPartitionConfig.taloscontainers">CPUPartitionRoot</a> |Host CPUs for containers declared via `ContainerConfig`.<br><br>Optional; omitting it leaves the root unrestricted.  | |
|`virtualMachines` |<a href="#CPUPartitionConfig.virtualMachines">CPUPartitionVirtualMachines</a> |Host CPUs for virtual machines declared via `VirtualMachineConfig`, optionally<br>divided into named slices.<br><br>Optional; omitting it leaves the root unrestricted and declares no slice.  | |




## init {#CPUPartitionConfig.init}

CPUPartitionRoot bounds one fixed root to a set of host CPUs.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`cpus` |string |Host CPUs the root may run on, as a Linux CPU list, e.g. `0-1` or `0,2-3`.<br><br>Host CPU IDs are the kernel's logical CPU numbers, SMT threads included. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
cpus: 0-1
{{< /highlight >}}</details> | |






## system {#CPUPartitionConfig.system}

CPUPartitionRoot bounds one fixed root to a set of host CPUs.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`cpus` |string |Host CPUs the root may run on, as a Linux CPU list, e.g. `0-1` or `0,2-3`.<br><br>Host CPU IDs are the kernel's logical CPU numbers, SMT threads included. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
cpus: 0-1
{{< /highlight >}}</details> | |






## podruntime {#CPUPartitionConfig.podruntime}

CPUPartitionRoot bounds one fixed root to a set of host CPUs.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`cpus` |string |Host CPUs the root may run on, as a Linux CPU list, e.g. `0-1` or `0,2-3`.<br><br>Host CPU IDs are the kernel's logical CPU numbers, SMT threads included. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
cpus: 0-1
{{< /highlight >}}</details> | |






## kubepods {#CPUPartitionConfig.kubepods}

CPUPartitionRoot bounds one fixed root to a set of host CPUs.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`cpus` |string |Host CPUs the root may run on, as a Linux CPU list, e.g. `0-1` or `0,2-3`.<br><br>Host CPU IDs are the kernel's logical CPU numbers, SMT threads included. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
cpus: 0-1
{{< /highlight >}}</details> | |






## taloscontainers {#CPUPartitionConfig.taloscontainers}

CPUPartitionRoot bounds one fixed root to a set of host CPUs.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`cpus` |string |Host CPUs the root may run on, as a Linux CPU list, e.g. `0-1` or `0,2-3`.<br><br>Host CPU IDs are the kernel's logical CPU numbers, SMT threads included. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
cpus: 0-1
{{< /highlight >}}</details> | |






## virtualMachines {#CPUPartitionConfig.virtualMachines}

CPUPartitionVirtualMachines bounds the virtual machine root and divides it into slices.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`cpus` |string |Host CPUs the virtual machine root may run on, as a Linux CPU list.<br><br>Every named slice is a subset of it; virtual machines selecting no slice run on<br>what the slices leave of it. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
cpus: 4-7
{{< /highlight >}}</details> | |
|`slices` |<a href="#CPUPartitionConfig.virtualMachines.slices.">[]CPUPartitionSlice</a> |Named, pairwise disjoint subsets of `cpus` a `VirtualMachineConfig` selects with<br>`cpu.slice`. The name `shared` is reserved for the remainder.<br><br>A configuration patch replaces this list as a whole rather than appending to it.  | |




### slices[] {#CPUPartitionConfig.virtualMachines.slices.}

CPUPartitionSlice is one named subset of the virtual machine root.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of the slice, unique within the document.<br><br>Follows the virtual machine name rule: between 1 and 63 ASCII letters, digits<br>and hyphens. `shared` is reserved. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
name: database
{{< /highlight >}}</details> | |
|`cpus` |string |Host CPUs of the slice, as a Linux CPU list. Must be a subset of the virtual<br>machine root and disjoint from every other slice. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
cpus: 6-7
{{< /highlight >}}</details> | |
|`exclusive` |bool |Reserve the slice for a single virtual machine.<br><br>An exclusive slice must not overlap any other root, all of which must then be<br>explicitly bounded, and at most one virtual machine may select it, whether it is<br>running or not. Its CPUs stay reserved while no virtual machine selects it.<br><br>Optional; defaults to false.  | |










