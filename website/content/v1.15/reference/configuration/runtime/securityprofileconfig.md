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
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`workloadIsolation` |bool |Enable workload isolation (run the container plane inside the sandbox namespace).  | |
|`ignoreCtrlAltDelete` |bool |Ignore Ctrl-Alt-Delete instead of rebooting the machine.  | |






