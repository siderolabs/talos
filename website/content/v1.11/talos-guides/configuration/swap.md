---
title: "Swap"
description: "Guide on managing swap devices and zswap configuration in Talos Linux."
---

This guide provides an overview of the swap management features in Talos Linux.

## Overview

Swap devices are used to extend the available memory on a system by allowing the kernel to move inactive pages from RAM to disk.
Swap might be useful to free up memory when running memory-intensive workloads, but it can also lead to performance degradation if used excessively.
On other hand, moving inactive pages to swap can allow Linux to use more memory for buffers and caches, which can improve performance for workloads that benefit from caching.

Zswap is a compressed cache for swap pages that can help reduce the performance impact of swapping by keeping frequently accessed pages in memory.
Swap and zswap can be used together, but they can also be configured independently.

Swap and zswap are disabled by default in Talos, but can be enabled through the configuration.

## Swap Devices

Swap devices can be configured in the [Talos machine configuration]({{< relref "../../reference/configuration/block/swapvolumeconfig" >}}) similar to how [User Volumes]({{< relref "disk-management#" >}}) are configured.
As swap devices contain memory pages, it is recommended to enable disk encryption for swap devices to prevent sensitive data from being written to disk in plaintext.
It is also recommended to use a separate disk for swap devices to avoid performance degradation on the system disk and other workloads.

For example, to configure a swap device on a NVMe disk of size 4GiB, using static key for encryption, the following configuration patch can be used:

```yaml
apiVersion: v1alpha1
kind: SwapVolumeConfig
name: swap1
provisioning:
    diskSelector:
        match: 'disk.transport == "nvme"'
    minSize: 4GiB
    maxSize: 4GiB
encryption:
    provider: luks2
    keys:
        - slot: 0
          tpm: {}
        - slot: 1
          static:
            passphrase: topsecret
```

Talos Linux will automatically provision the partition on the disk, label it as `s-swap1`, encrypt it using the provided key, and enable it as a swap device.

Current swap status can be checked using `talosctl get swap` command:

```shell
$ talosctl -n 172.20.0.5 get swap
NODE         NAMESPACE   TYPE         ID               VERSION   DEVICE           SIZE      USED     PRIORITY
172.20.0.5   runtime     SwapStatus   /dev/nvme0n2p2   1         /dev/nvme0n2p2   3.9 GiB   100 MiB  -2
```

Removing a `SwapVolumeConfig` document will remove the swap device from the system, but the partition will remain on the disk.
To

## Zswap

Zswap is a compressed cache for swap pages that can help reduce the performance impact of swapping by keeping frequently accessed pages in memory.
Zswap can be enabled in the [Talos machine configuration]({{< relref "../../reference/configuration/block/zswapconfig" >}}):

```yaml
apiVersion: v1alpha1
kind: ZswapConfig
maxPoolPercent: 20
```

This configuration enables zswap with a maximum pool size of 20% of the total system memory.
To check the current zswap status, you can use the `talosctl get zswapstatus` command:

```shell
$ talosctl -n 172.20.0.5 get zswapstatus
NODE         NAMESPACE   TYPE          ID      VERSION   TOTAL SIZE   STORED PAGES   WRITTEN BACK   POOL LIMIT HIT
172.20.0.5   runtime     ZswapStatus   zswap   1         0 B          0              0              0
```

Removing a `ZswapConfig` document will disable zswap on the system.
