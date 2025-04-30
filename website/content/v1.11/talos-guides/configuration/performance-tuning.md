---
title: "Performance Tuning"
description: "In this guide, we'll describe various performance tuning knobs available."
---

Talos Linux tries to strike a balance between performance and security/efficiency.
However, there are some performance tuning knobs available to adjust the system to your needs.
With any performance tuning, it's essential to measure the impact of the changes and ensure they don't introduce security vulnerabilities.

> Note: Most of the suggestions below apply to bare metal machines, but some of them might be useful for VMs as well.

If you find more performance tuning knobs, please let us know by editing this document.

## Kernel Parameters

Talos Linux kernel parameters can be adjusted in the following ways:

* temporary, one-time adjustments can be done via console access, and editing the kernel command line in the bootloader (doesn't work for Secure Boot enabled systems)
* on initial install (when booting off ISO/PXE), `.machine.install.extraKernelArgs` can be used to set kernel parameters
* after the initial install (or when booting off a disk image), `.machine.install.extraKernelArgs` changes require a no-op upgrade (e.g. to the same version of Talos) to take effect

### CPU Scaling

Talos Linux uses the `schedutil` [CPU scaling governor](https://docs.kernel.org/admin-guide/pm/cpufreq.html) by default, for maximum performance, you can switch to the `performance` governor:

```text
cpufreq.default_governor=performance
```

### Processor Sleep States

Modern processors support various sleep states to save power, but they might introduce latency when transitioning back to the active state.

#### AMD

For maximum performance (and lower latency), use `active` mode of the [amd-pstate driver](https://docs.kernel.org/admin-guide/pm/amd-pstate.html):

```text
amd_pstate=active
```

#### Intel

For maximum performance (and lower latency), disable the `intel_idle` driver:

```text
intel_idle.max_cstate=0
```

### Hardware Vulnerabilities

Modern processors have various [security vulnerabilities](https://docs.kernel.org/admin-guide/hw-vuln/index.html) that require software/microcode mitigations.
These mitigations might have a performance impact, and some of them can be disabled if you are willing to take the risk.

First of all, ensure that Talos system extensions `amd-ucode` and `intel-ucode` are installed (and using latest version of Talos Linux).
Linux kernel will load the microcode updates on early boot, and for some processors, it might reduce the performance impact of the mitigations.
The availability of microcode updates depends on the processor model.

The kernel command line argument `mitigations` can be used to disable all mitigations at once (not recommended from security point of view):

```text
mitigations=off
```

There is also a way to disable specific mitigations, see [Kernel documentation](https://docs.kernel.org/admin-guide/hw-vuln/index.html) for more details.

### I/O

For Talos Linux before version 1.8.2, the I/O performance can be improved by setting `iommu.strict=0`, for later versions this is a default setting.

Performance can be further improved at some cost of security by bypassing the I/O memory management unit (IOMMU) for DMA:

```text
iommu.passthrough=1
```
