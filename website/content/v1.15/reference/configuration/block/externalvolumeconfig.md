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

{{< highlight yaml >}}
apiVersion: v1alpha1
kind: ExternalVolumeConfig
name: mount2 # Name of the mount.
filesystemType: nfs # Filesystem type.
# The mount describes additional mount options.
mount:
    # NFS mount options.
    nfs:
        server: 10.0.0.10 # NFS server hostname or IP address.
        path: /export # Absolute path of the NFS export.
        version: "4.1" # NFS protocol version.
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
|`disableAccessTime` |bool |If true, disable file access time updates.  | |
|`secure` |bool |Enable secure mount options (nosuid, nodev, noexec).<br><br>Defaults to true for better security.  | |
|`virtiofs` |<a href="#ExternalVolumeConfig.mount.virtiofs">VirtiofsMountSpec</a> |Virtiofs mount options.  | |
|`nfs` |<a href="#ExternalVolumeConfig.mount.nfs">NFSMountSpec</a> |NFS mount options.  | |




### virtiofs {#ExternalVolumeConfig.mount.virtiofs}

VirtiofsMountSpec describes Virtiofs mount options.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`tag` |string |Selector tag for the Virtiofs mount.  | |






### nfs {#ExternalVolumeConfig.mount.nfs}

NFSMountSpec describes NFS mount options.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`server` |string |NFS server hostname or IP address.  | |
|`path` |string |Absolute path of the NFS export.  | |
|`version` |NFSVersion |NFS protocol version.  |`3`<br />`4`<br />`4.1`<br />`4.2`<br /> |
|`port` |uint16 |NFS server port. If unset, the kernel default is used.  | |
|`transport` |NFSTransport |NFS transport protocol. If unset, the kernel default is used.  |`tcp`<br />`tcp6`<br />`udp`<br />`udp6`<br /> |
|`mountPort` |uint16 |NFS mount protocol port. Only valid with NFSv3. If unset, rpcbind discovery is used.  | |
|`mountTransport` |NFSTransport |NFS mount transport protocol. Only valid with NFSv3. Must use the same address family as<br>`transport`. If unset, the kernel default is used.  |`tcp`<br />`tcp6`<br />`udp`<br />`udp6`<br /> |
|`locking` |NFSLocking |NFSv3 locking mode. Defaults to local because Talos does not run rpc.statd by default.  |`local`<br />`remote`<br /> |
|`recovery` |NFSRecovery |Recovery behavior after an NFS request times out. Soft modes can risk data corruption.  |`hard`<br />`soft`<br />`soft-error`<br /> |
|`timeout` |uint32 |NFS request timeout in deciseconds.  | |
|`retransmissions` |uint32 |Number of NFS request retransmissions before recovery action is taken.  | |
|`readSize` |uint32 |Maximum NFS read request payload in bytes. Must be a multiple of 1024 between 1024 and 1048576.  | |
|`writeSize` |uint32 |Maximum NFS write request payload in bytes. Must be a multiple of 1024 between 1024 and 1048576.  | |
|`connections` |uint8 |Number of TCP connections to the NFS server. Must be between 1 and 16.  | |
|`reservedPort` |bool |Use a privileged source port. The kernel default is used when unset.  | |
|`security` |NFSSecurity |NFS RPC security flavor. Kerberos flavors are not supported because Talos does not run rpc.gssd.  |`none`<br />`sys`<br /> |










