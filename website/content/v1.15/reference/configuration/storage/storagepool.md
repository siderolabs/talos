---
description: |
    StoragePool defines a directory storage pool on a configured filesystem volume.
    Defines a named storage pool backed by a UserVolumeConfig, ExistingVolumeConfig,
    or ExternalVolumeConfig. The backing volume must be writable and filesystem-backed.
    Only one pool may reference a backing volume. The pool directory is named after
    the pool beneath the volume mount target. Removing the configuration stops and
    undefines the pool, but never deletes its files or backing volume.
    Requires the libvirtd system extension. Changing the backing volume changes
    the pool target; it does not migrate existing disk images.
title: StoragePool
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: StoragePool
name: vm-images # Pool name: 1-63 ASCII letters, digits, hyphens or underscores, starting with a letter or digit.
# Reference to the writable filesystem volume backing this pool.
volume:
    name: u-vm-data # Runtime volume ID: `u-<name>` for UserVolumeConfig, `e-<name>` for ExistingVolumeConfig,
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Pool name: 1-63 ASCII letters, digits, hyphens or underscores, starting with a letter or digit.  | |
|`volume` |<a href="#StoragePool.volume">StoragePoolVolume</a> |Reference to the writable filesystem volume backing this pool.  | |




## volume {#StoragePool.volume}

StoragePoolVolume references a backing volume by its runtime volume ID.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Runtime volume ID: `u-<name>` for UserVolumeConfig, `e-<name>` for ExistingVolumeConfig,<br>or `x-<name>` for ExternalVolumeConfig, where `<name>` is the document name.<br>The prefix is required, even when the document name is unique.  | |








