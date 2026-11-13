---
description: |
    WorkloadResourceConfig is a workload resource config document.
    WorkloadResourceConfig splits the host memory between the Talos workload roots.

    `reserved.memory` stays with the host (`/init`, `/system`, `/podruntime` and the kernel); each
    root gets an aggregate memory ceiling of a whole percentage of the rest. The roots are
    `kubepods`, `taloscontainers` and `virtualMachines`; every root is optional, and the
    percentages must not add up to more than 100%.
    Limits are resolved against the host memory at runtime and follow memory hotplug. They are
    rounded down to the host page size before they are applied, and bound RAM only (including
    page cache charged to the root): swap and hugepages are not limited by them.

    Talos exclusively owns `memory.max` on `/taloscontainers` and `/virtualmachines.partition`,
    even without this document. An omitted root, `kubepods` included, is capped at the host memory
    minus `reserved.memory`, which defaults to the kubelet's default `systemReserved.memory`
    (512Mi on control plane nodes, 384Mi on workers). Individual child limits still apply.
    Kubelet `system-reserved` and `kube-reserved` enforcement on these roots and on
    `/system/extensions`, whose `pids.max` Talos also owns, is rejected, compressible included.
    Static validation cannot inspect external kubelet drop-in files;
    drop-in directories are rejected while this document sets any limit.
    Lowering a limit below current usage makes the kernel reclaim and OOM-kill within that root,
    which can terminate containers or virtual machines.

    The `kubepods` cap is applied by the kubelet, which stays the only writer of its cgroup:
    Talos sets `systemReserved.memory` to `reserved.memory`, or derives it from the `kubepods`
    limit. A limit makes `systemReserved.memory` depend on the host memory, so a memory hotplug
    restarts the kubelet; without a limit it does not. Changing either value restarts the kubelet,
    and a limit below the memory used by pods (control plane static pods included) causes
    OOM kills. The schedulable Node Allocatable is the limit minus the hard eviction
    threshold and any hugepage capacity, so it is lower than the limit. With the limit,
    `systemReserved.memory`, `kubeReserved.memory`, the equivalent kubelet command line flags,
    drop-in directories and the `Static` memory manager policy are rejected. Without it, an
    explicit `systemReserved.memory`, the `--system-reserved` flag or a drop-in file overrides
    `reserved.memory` for the kubelet, and keeping them consistent is the user's responsibility.
    The limit is inactive while Kubernetes is not configured on the machine; the other roots are
    enforced regardless.

    In container mode the cgroups are nested, so the limits are not applied; `reserved.memory`
    still sets the kubelet `systemReserved.memory`.
title: WorkloadResourceConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: WorkloadResourceConfig
# Resources left out of the split for the host.
reserved:
    memory: 2GiB # Host memory left out of the split. Defaults to the kubelet's default
# Limits for Kubernetes pods.
kubepods:
    # Memory limits of the root.
    memory:
        limit: 50% # Aggregate memory ceiling of the root, as a whole percentage of the host memory
# Limits for containers declared via `ContainerConfig`.
taloscontainers:
    # Memory limits of the root.
    memory:
        limit: 10% # Aggregate memory ceiling of the root, as a whole percentage of the host memory
# Limits for virtual machines.
virtualMachines:
    # Memory limits of the root.
    memory:
        limit: 40% # Aggregate memory ceiling of the root, as a whole percentage of the host memory
{{< /highlight >}}

{{< highlight yaml >}}
apiVersion: v1alpha1
kind: WorkloadResourceConfig
# Limits for virtual machines.
virtualMachines:
    # Memory limits of the root.
    memory:
        limit: 80% # Aggregate memory ceiling of the root, as a whole percentage of the host memory
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`reserved` |<a href="#WorkloadResourceConfig.reserved">WorkloadReserved</a> |Resources left out of the split for the host.  | |
|`kubepods` |<a href="#WorkloadResourceConfig.kubepods">WorkloadResourceRoot</a> |Limits for Kubernetes pods.  | |
|`taloscontainers` |<a href="#WorkloadResourceConfig.taloscontainers">WorkloadResourceRoot</a> |Limits for containers declared via `ContainerConfig`.  | |
|`virtualMachines` |<a href="#WorkloadResourceConfig.virtualMachines">WorkloadResourceRoot</a> |Limits for virtual machines.  | |




## reserved {#WorkloadResourceConfig.reserved}

WorkloadReserved holds the resources left for the host.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`memory` |ByteSize |Host memory left out of the split. Defaults to the kubelet's default<br>`systemReserved.memory`: 512Mi on control plane nodes, 384Mi on workers.<br><br>It is not written to any cgroup: it is the room left for the host processes<br>and the kernel. Talos passes it to the kubelet as `systemReserved.memory`<br>unless `kubepods` sets a limit. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
memory: 2GiB
{{< /highlight >}}</details> | |






## kubepods {#WorkloadResourceConfig.kubepods}

WorkloadResourceRoot holds the limits of one workload root.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`memory` |<a href="#WorkloadResourceConfig.kubepods.memory">WorkloadMemoryResource</a> |Memory limits of the root.  | |




### memory {#WorkloadResourceConfig.kubepods.memory}

WorkloadMemoryResource bounds the memory of one workload root.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`limit` |PercentageSize |Aggregate memory ceiling of the root, as a whole percentage of the host memory<br>minus `reserved.memory`. It must be positive. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
limit: 40%
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
|`limit` |PercentageSize |Aggregate memory ceiling of the root, as a whole percentage of the host memory<br>minus `reserved.memory`. It must be positive. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
limit: 40%
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
|`limit` |PercentageSize |Aggregate memory ceiling of the root, as a whole percentage of the host memory<br>minus `reserved.memory`. It must be positive. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
limit: 40%
{{< /highlight >}}</details> | |










