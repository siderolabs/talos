---
description: |
    VirtualMachineProfileConfig declares a named amd64 guest ABI.
    Profiles bundle the guest machine type and CPU model. Use a versioned machine
    type and a CPU baseline tested on every host for migration. Aliases drift as QEMU
    changes. A named CPU model is not a migration guarantee across arbitrary hosts.
title: VirtualMachineProfileConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: VirtualMachineProfileConfig
name: rack-a-compat # Profile name. Builtin names and the compat- prefix are reserved.
machine: pc-q35-9.2 # q35 or i440fx machine family, preferably a versioned type.
# Guest CPU model and feature policy.
cpu:
    mode: custom # host-model, host-passthrough or custom.
    model: Haswell-noTSX-IBRS # Named CPU model; required only for custom.
    # Explicit feature policies for a custom model.
    features:
        - name: pku # CPU feature name.
          policy: disable # require, disable, optional, force or forbid.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Profile name. Builtin names and the compat- prefix are reserved.  | |
|`machine` |string |q35 or i440fx machine family, preferably a versioned type.  | |
|`cpu` |<a href="#VirtualMachineProfileConfig.cpu">VirtualMachineProfileCPU</a> |Guest CPU model and feature policy.  | |




## cpu {#VirtualMachineProfileConfig.cpu}

VirtualMachineProfileCPU selects the guest-visible CPU.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`mode` |VirtualMachineProfileCPUMode |host-model, host-passthrough or custom.  |`host-model`<br />`host-passthrough`<br />`custom`<br /> |
|`model` |string |Named CPU model; required only for custom.  | |
|`fallback` |string |allow or forbid (default forbid) for custom model fallback.  | |
|`features` |<a href="#VirtualMachineProfileConfig.cpu.features.">[]VirtualMachineProfileCPUFeature</a> |Explicit feature policies for a custom model.  | |




### features[] {#VirtualMachineProfileConfig.cpu.features.}

VirtualMachineProfileCPUFeature selects one CPU flag.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |CPU feature name.  | |
|`policy` |VirtualMachineProfileCPUFeaturePolicy |require, disable, optional, force or forbid.  |`require`<br />`disable`<br />`optional`<br />`force`<br />`forbid`<br /> |










