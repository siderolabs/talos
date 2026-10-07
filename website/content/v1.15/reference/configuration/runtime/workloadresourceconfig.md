---
description: |
    WorkloadResourceConfig is a workload resource config document.
    WorkloadResourceConfig declares an aggregate memory ceiling for each Talos workload root.

    The roots are `kubepods`, `taloscontainers` and `virtualMachines`; every root is optional
    and independent of the others. Omitting a root removes its aggregate cap; individual child
    limits still apply.
    A limit must be positive; it is rounded down to the host page size before it is applied,
    and bounds RAM only (including page cache charged to the root): swap and hugepages are
    not limited by it.

    Talos exclusively owns `memory.max` on `/taloscontainers` and `/virtualmachines.partition`,
    even without this document. An omitted limit (zero internally) means an unlimited aggregate cap.
    Kubelet reserved-memory enforcement on these roots is rejected; CPU-only or compressible
    enforcement is allowed. Static validation cannot inspect external kubelet drop-in files;
    drop-in directories are rejected while any workload cap is active.
    Lowering a limit below current usage makes the kernel reclaim and OOM-kill within that root,
    which can terminate containers or virtual machines.

    The `kubepods` limit is applied by the kubelet, which stays the only writer of its cgroup:
    Talos derives `systemReserved.memory` from the limit, so changing it restarts the kubelet,
    and a limit below the memory used by pods (control plane static pods included) causes
    OOM kills. The schedulable Node Allocatable is the limit minus the hard eviction
    threshold and any hugepage capacity, so it is lower than the limit. Setting
    `systemReserved.memory`, the equivalent kubelet command line flags or the `Static` memory
    manager policy together with the limit is rejected. The limit is inactive while Kubernetes
    is not configured on the machine; the other roots are enforced regardless.

    In container mode the document is validated but declares no policy.
title: WorkloadResourceConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: WorkloadResourceConfig
# Limits for Kubernetes pods.
kubepods:
    # Memory limits of the root.
    memory:
        limit: 16GiB # Aggregate memory ceiling of the root, in bytes.
# Limits for containers declared via `ContainerConfig`.
taloscontainers:
    # Memory limits of the root.
    memory:
        limit: 4GiB # Aggregate memory ceiling of the root, in bytes.
# Limits for virtual machines.
virtualMachines:
    # Memory limits of the root.
    memory:
        limit: 32GiB # Aggregate memory ceiling of the root, in bytes.
{{< /highlight >}}

{{< highlight yaml >}}
apiVersion: v1alpha1
kind: WorkloadResourceConfig
# Limits for virtual machines.
virtualMachines:
    # Memory limits of the root.
    memory:
        limit: 48GiB # Aggregate memory ceiling of the root, in bytes.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`kubepods` |<a href="#WorkloadResourceConfig.kubepods">WorkloadResourceRoot</a> |Limits for Kubernetes pods.  | |
|`taloscontainers` |<a href="#WorkloadResourceConfig.taloscontainers">WorkloadResourceRoot</a> |Limits for containers declared via `ContainerConfig`.  | |
|`virtualMachines` |<a href="#WorkloadResourceConfig.virtualMachines">WorkloadResourceRoot</a> |Limits for virtual machines.  | |




## kubepods {#WorkloadResourceConfig.kubepods}

WorkloadResourceRoot holds the limits of one workload root.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`memory` |<a href="#WorkloadResourceConfig.kubepods.memory">WorkloadMemoryResource</a> |Memory limits of the root.  | |




### memory {#WorkloadResourceConfig.kubepods.memory}

WorkloadMemoryResource bounds the memory of one workload root.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`limit` |ByteSize |Aggregate memory ceiling of the root, in bytes.<br><br>The value can be expressed in human readable format, e.g. 16GiB, and must be positive. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
limit: 16GiB
{{< /highlight >}}</details> | |








## taloscontainers {#WorkloadResourceConfig.taloscontainers}

WorkloadResourceRoot holds the limits of one workload root.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`memory` |<a href="#WorkloadResourceConfig.taloscontainers.memory">WorkloadMemoryResource</a> |Memory limits of the root.  | |




### memory {#WorkloadResourceConfig.taloscontainers.memory}

WorkloadMemoryResource bounds the memory of one workload root.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`limit` |ByteSize |Aggregate memory ceiling of the root, in bytes.<br><br>The value can be expressed in human readable format, e.g. 16GiB, and must be positive. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
limit: 16GiB
{{< /highlight >}}</details> | |








## virtualMachines {#WorkloadResourceConfig.virtualMachines}

WorkloadResourceRoot holds the limits of one workload root.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`memory` |<a href="#WorkloadResourceConfig.virtualMachines.memory">WorkloadMemoryResource</a> |Memory limits of the root.  | |




### memory {#WorkloadResourceConfig.virtualMachines.memory}

WorkloadMemoryResource bounds the memory of one workload root.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`limit` |ByteSize |Aggregate memory ceiling of the root, in bytes.<br><br>The value can be expressed in human readable format, e.g. 16GiB, and must be positive. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
limit: 16GiB
{{< /highlight >}}</details> | |










