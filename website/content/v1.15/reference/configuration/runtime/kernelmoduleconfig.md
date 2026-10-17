---
description: |
    KernelModuleConfig is a config document to configure a Linux kernel module to load.
    Modules are loaded in the order of the documents in the machine configuration,
    after the modules required by Talos itself.
    The kernel doesn't load modules on demand, so if a module loads other modules while probing a device
    (e.g. `earth-pt3` loads `tc90522`, `qm1d1c0042` and `mxl301rf`), list those modules before it.
title: KernelModuleConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: KernelModuleConfig
name: btrfs # Module name.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Module name.  | |
|`parameters` |[]string |Module parameters, changes applied after reboot.  | |






