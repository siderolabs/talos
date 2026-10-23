---
description: |
    KernelModuleConfig is a config document to configure a Linux kernel module to load.
    Listed modules are loaded at boot with the given parameters.

    Modules requested by the kernel on demand (e.g. a driver loading a helper module while probing a device)
    are loaded automatically, whether listed or not, so there is no need to list such modules or to order them;
    the parameters are applied if the module is listed.
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






