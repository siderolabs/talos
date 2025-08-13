---
title: "Disk Management"
description: "Guide on managing disks"
aliases:
  - ../../guides/disk-management
---

This guide provides an overview of the disk management features in Talos Linux.

## Disk and Volume Discovery

See [Disk Layout]({{< relref "layout" >}}) for details on the disk layout and how to observe discovered disks and volumes.

## Volume Management

Talos Linux implements disk management through the concept of volumes.
A volume represents a provisioned, located, mounted, or unmounted entity, such as a disk, partition, or a directory/overlay mount.

Talos Linux has [built-in (system) volumes]({{< relref "system" >}}), which can be partially configured by the user, and user-defined volumes, which are fully configurable by the user.
User volumes come in several flavors:

* [User Volumes]({{< relref "user" >}}) - for dynamically allocated local storage for Kubernetes workloads.
* [Raw Volumes]({{< relref "raw" >}}) - for allocating unformatted storage (e.g. to be used with CSIs).
* [Existing Volumes]({{< relref "existing" >}}) - for mounting pre-existing partitions or disks.

For information on allocating swap space, see [Swap Management]({{< relref "../swap" >}}).

Configuration documents related to volume management are located in the [`block` group]({{< relref "../../../reference/configuration/block" >}}), see [common configuration]({{< relref "common" >}}) for common fields
in volume configuration documents.
