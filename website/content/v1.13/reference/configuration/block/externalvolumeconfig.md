---
description: |
    ExternalVolumeConfig is an external disk mount configuration document.
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
    # Virtiofs mount options.
    virtiofs:
        tag: Data # Selector tag for the Virtiofs mount.
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
|`virtiofs` |<a href="#ExternalVolumeConfig.mount.virtiofs">VirtiofsMountSpec</a> |Virtiofs mount options.  | |




### virtiofs {#ExternalVolumeConfig.mount.virtiofs}

VirtiofsMountSpec describes Virtiofs mount options.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`tag` |string |Selector tag for the Virtiofs mount.  | |










