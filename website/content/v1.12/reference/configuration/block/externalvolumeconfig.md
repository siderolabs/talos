---
description: |
    ExternalVolumeConfig is a external disk mount configuration document.
    External volumes allow to mount volumes that were created outside of Talos,
    over the network or API. Volume will be mounted under `/var/mnt/<name>`.
    The external volume config name should not conflict with user volume names.
title: ExternalVolumeConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: ExternalVolumeConfig
name: mount1 # Name of the mount.
filesystemType: virtiofs # Filesystem type.
# The mount describes additional mount options.
mount:
    source: Data # Source of the volume.
{{< /highlight >}}

{{< highlight yaml >}}
apiVersion: v1alpha1
kind: ExternalVolumeConfig
name: mount1 # Name of the mount.
filesystemType: virtiofs # Filesystem type.
# The mount describes additional mount options.
mount:
    source: 10.2.21.1:/backups # Source of the volume.
    # NFS mount options.
    nfs:
        version: "4.2" # NFS version to use.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of the mount.<br><br>Name might be between 1 and 34 characters long and can only contain:<br>lowercase and uppercase ASCII letters, digits, and hyphens.  | |
|`filesystemType` |FilesystemType |Filesystem type.  |`virtiofs`<br />`nfs`<br /> |
|`mount` |<a href="#ExternalVolumeConfig.mount">ExternalMountSpec</a> |The mount describes additional mount options.  | |




## mount {#ExternalVolumeConfig.mount}

ExternalMountSpec describes how the external volume is mounted.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`readOnly` |bool |Mount the volume read-only.  | |
|`source` |string |Source of the volume.  | |
|`nfs` |<a href="#ExternalVolumeConfig.mount.nfs">NFSMountSpec</a> |NFS mount options.  | |




### nfs {#ExternalVolumeConfig.mount.nfs}

NFSMountSpec describes NFS mount options.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`version` |NFSVersionType |NFS version to use.  |`4.2`<br />`4.1`<br />`4`<br />`3`<br />`2`<br /> |










