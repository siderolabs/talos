---
title: "Existing Volumes"
description: "Configuring existing volumes to mount migrated or pre-existing partitions and disks."
weight: 50
---

Existing volumes allow mounting pre-existing partitions or disks that are already formatted and contain data.
This is useful for migrating data from another system or reusing existing disks without reformatting them.

Existing volumes match a partition or a disk using a [volume selector]({{< relref "common#volume-selector" >}}) expression.

Existing volumes are mounted under `/var/mnt/<volume-name>`, and this location gets automatically propagated into the `kubelet` container to provide additional features like `subPath` mounts.

> Note: If you need to allocate a volume to be mounted to a container, please see [User Volumes]({{< relref "user" >}}) guide.

### Declaring Existing Volumes

To declare an existing volume, append the following [document]({{< relref "../../../reference/configuration/block/existingvolumeconfig" >}}) to the machine configuration:

```yaml
# existing-volume.patch.yaml
apiVersion: v1alpha1
kind: ExistingVolumeConfig
name: my-data-volume
discovery:
    volumeSelector:
        match: volume.partition_label == "MY-DATA"

```

For example, this machine configuration patch can be applied using the following command:

```bash
talosctl --nodes <NODE> patch mc --patch @raw-volume.patch.yaml
```

In this example, a existing partition with partition label `MY-DATA` will be mounted as under `/var/mnt/my-data-volume`.

The status of the volume can be checked using the following command:

```bash
$ talosctl get volumestatus e-my-data-volume # note e- prefix
NODE         NAMESPACE   TYPE           ID                VERSION   TYPE        PHASE   LOCATION    SIZE
172.20.0.5   runtime     VolumeStatus   e-my-data-volume  1         partition   ready   /dev/sda1   2.0 GB
```

If the volume is no longer needed, it can be removed by deleting the `ExistingVolumeConfig` document from the machine configuration.
Talos will automatically unmount the volume, but it will not try to wipe the underlying data.
