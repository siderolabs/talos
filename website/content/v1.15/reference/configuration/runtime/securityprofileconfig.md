---
description: |
    SecurityProfileConfig is a node security profile configuration document.
    The security profile groups node-level security hardening features. Additional hardening options
    will be added to this document over time.

    Workload isolation runs the container runtime plane (CRI containerd, the kubelet, and all pods)
    inside a dedicated PID and mount namespace anchored by the `sandboxd` service, isolating them from
    `machined` (PID 1) and its file descriptors.

    `talosctl gen config` emits this document with `workloadIsolation: true` for Talos 1.14+, so new
    clusters are isolated by default; clusters upgraded from older versions do not have the document and
    keep the old (non-isolated) behavior unless it is added.

    Note: with workload isolation enabled, the deprecated in-tree Kubernetes iSCSI volume plugin does not
    work (the kubelet cannot reach the host iscsid across the sandbox); use a CSI driver instead.

    SELinux categories split the MCS categories between the workloads: the CRI labels Kubernetes containers
    with a random pair drawn from `c0-c767`, containers declared via `ContainerConfig` get a stable pair
    from `c768-c1007` derived from their name, and `c1008-c1023` are reserved for Talos (the kube-apiserver
    static pod runs at `s0:c1022,c1023`). Workloads with different categories cannot access each other's
    processes and files, even when they share an SELinux type. The kubelet gets `/sys/fs/selinux` and the
    containers contexts file, so that it labels volumes and its plugin directories.

    `talosctl gen config` emits this document with `selinuxCategories: true` for Talos 1.15+; clusters
    upgraded from older versions keep running their workloads without categories unless it is set.
    The categories only take effect when SELinux is enabled.

    By default, Ctrl-Alt-Delete reboots the machine. With `ignoreCtrlAltDelete: true`, Ctrl-Alt-Delete
    is logged and ignored. Ctrl-Alt-Delete still reboots the machine until a configuration with this
    option is applied. The option has no effect in container mode.
title: SecurityProfileConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: SecurityProfileConfig
workloadIsolation: true # Enable workload isolation (run the container plane inside the sandbox namespace).
selinuxCategories: true # Label workloads with SELinux MCS categories, split between the CRI, Talos containers and Talos itself.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`workloadIsolation` |bool |Enable workload isolation (run the container plane inside the sandbox namespace).  | |
|`ignoreCtrlAltDelete` |bool |Ignore Ctrl-Alt-Delete instead of rebooting the machine.  | |
|`selinuxCategories` |bool |Label workloads with SELinux MCS categories, split between the CRI, Talos containers and Talos itself.  | |






