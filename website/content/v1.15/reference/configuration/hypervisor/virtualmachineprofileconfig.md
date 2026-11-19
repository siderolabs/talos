---
description: |
    VirtualMachineProfileConfig declares a named guest ABI.
    Profiles bundle the guest machine type, CPU model and platform timers. Use a versioned machine
    type and a CPU baseline tested on every host for migration. Aliases drift as QEMU
    changes. A named CPU model is not a migration guarantee across arbitrary hosts.
title: VirtualMachineProfileConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: VirtualMachineProfileConfig
name: rack-a-compat # Profile name. Builtin names and the compat- prefix are reserved.
machine: pc-q35-9.2 # q35 or i440fx machine family on amd64, virt on arm64, preferably a versioned type.
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
|`machine` |string |q35 or i440fx machine family on amd64, virt on arm64, preferably a versioned type.  | |
|`cpu` |<a href="#VirtualMachineProfileConfig.cpu">VirtualMachineProfileCPU</a> |Guest CPU model and feature policy.  | |
|`timers` |<a href="#VirtualMachineProfileConfig.timers">VirtualMachineProfileTimers</a> |Platform timers presented to the guest.<br><br>Optional; omitting it hides the HPET, the RTC catches up on missed ticks and the PIT delays them.<br>These are x86 timers: arm64 guests get none of them, and enabling the HPET is rejected there.  | |




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








## timers {#VirtualMachineProfileConfig.timers}

VirtualMachineProfileTimers selects the guest-visible platform timers.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`hpet` |<a href="#VirtualMachineProfileConfig.timers.hpet">VirtualMachineProfileHPET</a> |Emulated HPET settings.<br><br>Optional; the HPET is hidden from the guest when this section is omitted.  | |
|`rtc` |<a href="#VirtualMachineProfileConfig.timers.rtc">VirtualMachineProfileRTC</a> |Emulated RTC settings.<br><br>Optional; the RTC catches up on missed ticks when this section is omitted.  | |
|`pit` |<a href="#VirtualMachineProfileConfig.timers.pit">VirtualMachineProfilePIT</a> |Emulated PIT settings.<br><br>Optional; the PIT delays missed ticks when this section is omitted.  | |




### hpet {#VirtualMachineProfileConfig.timers.hpet}

VirtualMachineProfileHPET describes the emulated High Precision Event Timer.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`enabled` |bool |Present an emulated HPET to the guest.<br><br>Disabled by default: modern guests use kvm-clock and the local APIC timer, and the<br>emulated HPET keeps QEMU's main thread waking at the guest's tick rate even after the<br>guest has stopped using it. Enable it only for guests that require an HPET.<br><br>Optional; defaults to disabled.  | |






### rtc {#VirtualMachineProfileConfig.timers.rtc}

VirtualMachineProfileRTC describes the emulated real-time clock.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`tickPolicy` |VirtualMachineProfileRTCTickPolicy |How the RTC delivers ticks the guest missed while it was descheduled.<br><br>catchup delivers missed ticks at a higher rate until the guest catches up (QEMU driftfix=slew).<br>Guests without an HPET, such as Windows, keep time from the RTC and drift without it.<br>delay delivers missed ticks at the normal rate, so guest time falls behind.<br><br>Optional; defaults to catchup.  |`catchup`<br />`delay`<br /> |






### pit {#VirtualMachineProfileConfig.timers.pit}

VirtualMachineProfilePIT describes the emulated programmable interval timer.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`tickPolicy` |VirtualMachineProfilePITTickPolicy |How the in-kernel PIT delivers ticks the guest missed while it was descheduled.<br><br>delay delivers missed ticks at the normal rate. discard drops them.<br><br>Optional; defaults to delay.  |`delay`<br />`discard`<br /> |










