---
description: |
    CPUPartitionConfig is a CPU partition config document.
    CPUPartitionConfig declares the host CPUs each fixed Talos workload root may run on.

    The roots are `init`, `system`, `podruntime`, `kubepods`, `taloscontainers` and
    `virtualMachines`. A root left out of the document is unrestricted; a root listed in
    the document must name at least one CPU. Roots may overlap each other.

    The document does not require virtual machines: omit `virtualMachines` to partition
    the host services and Kubernetes pods of an ordinary node.

    The virtual machine root may be divided into named, pairwise disjoint slices. An
    exclusive slice must not overlap any other root, and every other root must then be
    bounded explicitly.

    In container mode the document is validated but declares no policy.
title: CPUPartitionConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: CPUPartitionConfig
# Host CPUs for machined and the early boot services.
init:
    cpus: 0-1 # Host CPUs as a Linux CPU list, e.g. `0-1` or `0,2-3`.
# Host CPUs for Talos system services (apid, trustd, udevd, ...).
system:
    cpus: 0-1 # Host CPUs as a Linux CPU list, e.g. `0-1` or `0,2-3`.
# Host CPUs for the Kubernetes runtime components (containerd, the kubelet, etcd).
podruntime:
    cpus: 0-1 # Host CPUs as a Linux CPU list, e.g. `0-1` or `0,2-3`.
# Host CPUs for Kubernetes pods.
kubepods:
    cpus: 2-7 # Host CPUs as a Linux CPU list, e.g. `0-1` or `0,2-3`.
{{< /highlight >}}

{{< highlight yaml >}}
apiVersion: v1alpha1
kind: CPUPartitionConfig
# Host CPUs for machined and the early boot services.
init:
    cpus: 0-1 # Host CPUs as a Linux CPU list, e.g. `0-1` or `0,2-3`.
# Host CPUs for Talos system services (apid, trustd, udevd, ...).
system:
    cpus: 0-1 # Host CPUs as a Linux CPU list, e.g. `0-1` or `0,2-3`.
# Host CPUs for the Kubernetes runtime components (containerd, the kubelet, etcd).
podruntime:
    cpus: 0-1 # Host CPUs as a Linux CPU list, e.g. `0-1` or `0,2-3`.
# Host CPUs for Kubernetes pods.
kubepods:
    cpus: 2-3 # Host CPUs as a Linux CPU list, e.g. `0-1` or `0,2-3`.
# Host CPUs for containers declared via `ContainerConfig`.
taloscontainers:
    cpus: 0-1 # Host CPUs as a Linux CPU list, e.g. `0-1` or `0,2-3`.
# Host CPUs for virtual machines, optionally divided into named slices.
virtualMachines:
    cpus: 4-7 # Host CPUs of the virtual machine root, as a Linux CPU list.
    # Named, pairwise disjoint subsets of `cpus`.
    slices:
        - name: database # Unique slice name: 1 to 63 ASCII letters, digits and hyphens. `shared` is reserved.
          cpus: 6-7 # Host CPUs of the slice, as a Linux CPU list within the virtual machine root.
          exclusive: true # Keep the slice's CPUs out of every other root.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`init` |<a href="#CPUPartitionConfig.init">CPUPartitionRoot</a> |Host CPUs for machined and the early boot services.  | |
|`system` |<a href="#CPUPartitionConfig.system">CPUPartitionRoot</a> |Host CPUs for Talos system services (apid, trustd, udevd, ...).  | |
|`podruntime` |<a href="#CPUPartitionConfig.podruntime">CPUPartitionRoot</a> |Host CPUs for the Kubernetes runtime components (containerd, the kubelet, etcd).  | |
|`kubepods` |<a href="#CPUPartitionConfig.kubepods">CPUPartitionRoot</a> |Host CPUs for Kubernetes pods.  | |
|`taloscontainers` |<a href="#CPUPartitionConfig.taloscontainers">CPUPartitionRoot</a> |Host CPUs for containers declared via `ContainerConfig`.  | |
|`virtualMachines` |<a href="#CPUPartitionConfig.virtualMachines">CPUPartitionVirtualMachines</a> |Host CPUs for virtual machines, optionally divided into named slices.  | |




## init {#CPUPartitionConfig.init}

CPUPartitionRoot bounds one fixed root to a set of host CPUs.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`cpus` |string |Host CPUs as a Linux CPU list, e.g. `0-1` or `0,2-3`.<br><br>CPU IDs are the kernel's logical CPU numbers, SMT threads included. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
cpus: 0-1
{{< /highlight >}}</details> | |






## system {#CPUPartitionConfig.system}

CPUPartitionRoot bounds one fixed root to a set of host CPUs.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`cpus` |string |Host CPUs as a Linux CPU list, e.g. `0-1` or `0,2-3`.<br><br>CPU IDs are the kernel's logical CPU numbers, SMT threads included. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
cpus: 0-1
{{< /highlight >}}</details> | |






## podruntime {#CPUPartitionConfig.podruntime}

CPUPartitionRoot bounds one fixed root to a set of host CPUs.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`cpus` |string |Host CPUs as a Linux CPU list, e.g. `0-1` or `0,2-3`.<br><br>CPU IDs are the kernel's logical CPU numbers, SMT threads included. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
cpus: 0-1
{{< /highlight >}}</details> | |






## kubepods {#CPUPartitionConfig.kubepods}

CPUPartitionRoot bounds one fixed root to a set of host CPUs.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`cpus` |string |Host CPUs as a Linux CPU list, e.g. `0-1` or `0,2-3`.<br><br>CPU IDs are the kernel's logical CPU numbers, SMT threads included. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
cpus: 0-1
{{< /highlight >}}</details> | |






## taloscontainers {#CPUPartitionConfig.taloscontainers}

CPUPartitionRoot bounds one fixed root to a set of host CPUs.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`cpus` |string |Host CPUs as a Linux CPU list, e.g. `0-1` or `0,2-3`.<br><br>CPU IDs are the kernel's logical CPU numbers, SMT threads included. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
cpus: 0-1
{{< /highlight >}}</details> | |






## virtualMachines {#CPUPartitionConfig.virtualMachines}

CPUPartitionVirtualMachines bounds the virtual machine root and divides it into slices.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`cpus` |string |Host CPUs of the virtual machine root, as a Linux CPU list. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
cpus: 4-7
{{< /highlight >}}</details> | |
|`slices` |<a href="#CPUPartitionConfig.virtualMachines.slices.">[]CPUPartitionSlice</a> |Named, pairwise disjoint subsets of `cpus`.<br><br>A configuration patch replaces this list as a whole.  | |




### slices[] {#CPUPartitionConfig.virtualMachines.slices.}

CPUPartitionSlice is one named subset of the virtual machine root.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Unique slice name: 1 to 63 ASCII letters, digits and hyphens. `shared` is reserved. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
name: database
{{< /highlight >}}</details> | |
|`cpus` |string |Host CPUs of the slice, as a Linux CPU list within the virtual machine root. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
cpus: 6-7
{{< /highlight >}}</details> | |
|`exclusive` |bool |Keep the slice's CPUs out of every other root.  | |










