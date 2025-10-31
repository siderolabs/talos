---
description: |
    UserVolumeConfig is a user volume configuration document.
    User volume is automatically allocated as a partition on the specified disk
    and mounted under `/var/mnt/<name>`.
    The partition label is automatically generated as `u-<name>`.
title: UserVolumeConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: UserVolumeConfig
name: local-data # Name of the volume.
volumeType: directory # Volume type.

# # The encryption describes how the volume is encrypted.
# encryption:
#     provider: luks2 # Encryption provider to use for the encryption.
#     # Defines the encryption keys generation and storage method.
#     keys:
#         - slot: 0 # Key slot number for LUKS2 encryption.
#           # Key which value is stored in the configuration file.
#           static:
#             passphrase: exampleKey # Defines the static passphrase value.
#
#           # # KMS managed encryption key.
#           # kms:
#           #     endpoint: https://192.168.88.21:4443 # KMS endpoint to Seal/Unseal the key.
#         - slot: 1 # Key slot number for LUKS2 encryption.
#           # KMS managed encryption key.
#           kms:
#             endpoint: https://example-kms-endpoint.com # KMS endpoint to Seal/Unseal the key.
#     cipher: aes-xts-plain64 # Cipher to use for the encryption. Depends on the encryption provider.
#     blockSize: 4096 # Defines the encryption sector size.
#     # Additional --perf parameters for the LUKS2 encryption.
#     options:
#         - no_read_workqueue
#         - no_write_workqueue
{{< /highlight >}}

{{< highlight yaml >}}
apiVersion: v1alpha1
kind: UserVolumeConfig
name: local-data # Name of the volume.
volumeType: disk # Volume type.
# The provisioning describes how the volume is provisioned.
provisioning:
    # The disk selector expression.
    diskSelector:
        match: disk.transport == "nvme" # The Common Expression Language (CEL) expression to match the disk.

    # # The minimum size of the volume.
    # minSize: 2.5GiB

    # # The maximum size of the volume, if not specified the volume can grow to the size of the
    # maxSize: 50GiB
# The filesystem describes how the volume is formatted.
filesystem:
    type: xfs # Filesystem type. Default is `xfs`.
# The encryption describes how the volume is encrypted.
encryption:
    provider: luks2 # Encryption provider to use for the encryption.
    # Defines the encryption keys generation and storage method.
    keys:
        - slot: 0 # Key slot number for LUKS2 encryption.
          # Enable TPM based disk encryption.
          tpm: {}

          # # KMS managed encryption key.
          # kms:
          #     endpoint: https://192.168.88.21:4443 # KMS endpoint to Seal/Unseal the key.
        - slot: 1 # Key slot number for LUKS2 encryption.
          # Key which value is stored in the configuration file.
          static:
            passphrase: topsecret # Defines the static passphrase value.

          # # KMS managed encryption key.
          # kms:
          #     endpoint: https://192.168.88.21:4443 # KMS endpoint to Seal/Unseal the key.

    # # Cipher to use for the encryption. Depends on the encryption provider.
    # cipher: aes-xts-plain64

    # # Defines the encryption sector size.
    # blockSize: 4096

    # # Additional --perf parameters for the LUKS2 encryption.
    # options:
    #     - no_read_workqueue
    #     - no_write_workqueue
{{< /highlight >}}

{{< highlight yaml >}}
apiVersion: v1alpha1
kind: UserVolumeConfig
name: local-data # Name of the volume.
volumeType: partition # Volume type.
# The provisioning describes how the volume is provisioned.
provisioning:
    # The disk selector expression.
    diskSelector:
        match: disk.transport == "nvme" # The Common Expression Language (CEL) expression to match the disk.
    maxSize: 50GiB # The maximum size of the volume, if not specified the volume can grow to the size of the

    # # The minimum size of the volume.
    # minSize: 2.5GiB
# The filesystem describes how the volume is formatted.
filesystem:
    type: xfs # Filesystem type. Default is `xfs`.
# The encryption describes how the volume is encrypted.
encryption:
    provider: luks2 # Encryption provider to use for the encryption.
    # Defines the encryption keys generation and storage method.
    keys:
        - slot: 0 # Key slot number for LUKS2 encryption.
          # Enable TPM based disk encryption.
          tpm: {}

          # # KMS managed encryption key.
          # kms:
          #     endpoint: https://192.168.88.21:4443 # KMS endpoint to Seal/Unseal the key.
        - slot: 1 # Key slot number for LUKS2 encryption.
          # Key which value is stored in the configuration file.
          static:
            passphrase: topsecret # Defines the static passphrase value.

          # # KMS managed encryption key.
          # kms:
          #     endpoint: https://192.168.88.21:4443 # KMS endpoint to Seal/Unseal the key.

    # # Cipher to use for the encryption. Depends on the encryption provider.
    # cipher: aes-xts-plain64

    # # Defines the encryption sector size.
    # blockSize: 4096

    # # Additional --perf parameters for the LUKS2 encryption.
    # options:
    #     - no_read_workqueue
    #     - no_write_workqueue
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of the volume.<br><br>Name might be between 1 and 34 characters long and can only contain:<br>lowercase and uppercase ASCII letters, digits, and hyphens.  | |
|`volumeType` |VolumeType |Volume type.  |`directory`<br />`disk`<br />`partition`<br /> |
|`provisioning` |<a href="#UserVolumeConfig.provisioning">ProvisioningSpec</a> |The provisioning describes how the volume is provisioned.  | |
|`filesystem` |<a href="#UserVolumeConfig.filesystem">FilesystemSpec</a> |The filesystem describes how the volume is formatted.  | |
|`encryption` |<a href="#UserVolumeConfig.encryption">EncryptionSpec</a> |The encryption describes how the volume is encrypted.  | |




## provisioning {#UserVolumeConfig.provisioning}

ProvisioningSpec describes how the volume is provisioned.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`diskSelector` |<a href="#UserVolumeConfig.provisioning.diskSelector">DiskSelector</a> |The disk selector expression.  | |
|`grow` |bool |Should the volume grow to the size of the disk (if possible).  | |
|`minSize` |ByteSize |The minimum size of the volume.<br><br>Size is specified in bytes, but can be expressed in human readable format, e.g. 100MB. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
minSize: 2.5GiB
{{< /highlight >}}</details> | |
|`maxSize` |ByteSize |The maximum size of the volume, if not specified the volume can grow to the size of the<br>disk.<br><br>Size is specified in bytes, but can be expressed in human readable format, e.g. 100MB. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
maxSize: 50GiB
{{< /highlight >}}</details> | |




### diskSelector {#UserVolumeConfig.provisioning.diskSelector}

DiskSelector selects a disk for the volume.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`match` |Expression |The Common Expression Language (CEL) expression to match the disk. <details><summary>Show example(s)</summary>match disks with size between 120GB and 1TB:{{< highlight yaml >}}
match: disk.size > 120u * GB && disk.size < 1u * TB
{{< /highlight >}}match SATA disks that are not rotational and not system disks:{{< highlight yaml >}}
match: disk.transport == "sata" && !disk.rotational && !system_disk
{{< /highlight >}}</details> | |








## filesystem {#UserVolumeConfig.filesystem}

FilesystemSpec configures the filesystem for the volume.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`type` |FilesystemType |Filesystem type. Default is `xfs`.  |`ext4`<br />`xfs`<br /> |
|`projectQuotaSupport` |bool |Enables project quota support, valid only for 'xfs' filesystem.<br><br>Note: changing this value might require a full remount of the filesystem.  | |






## encryption {#UserVolumeConfig.encryption}

EncryptionSpec represents volume encryption settings.



{{< highlight yaml >}}
encryption:
    provider: luks2 # Encryption provider to use for the encryption.
    # Defines the encryption keys generation and storage method.
    keys:
        - slot: 0 # Key slot number for LUKS2 encryption.
          # Key which value is stored in the configuration file.
          static:
            passphrase: exampleKey # Defines the static passphrase value.

          # # KMS managed encryption key.
          # kms:
          #     endpoint: https://192.168.88.21:4443 # KMS endpoint to Seal/Unseal the key.
        - slot: 1 # Key slot number for LUKS2 encryption.
          # KMS managed encryption key.
          kms:
            endpoint: https://example-kms-endpoint.com # KMS endpoint to Seal/Unseal the key.
    cipher: aes-xts-plain64 # Cipher to use for the encryption. Depends on the encryption provider.
    blockSize: 4096 # Defines the encryption sector size.

    # # Additional --perf parameters for the LUKS2 encryption.
    # options:
    #     - no_read_workqueue
    #     - no_write_workqueue
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`provider` |EncryptionProviderType |Encryption provider to use for the encryption.  |`luks2`<br /> |
|`keys` |<a href="#UserVolumeConfig.encryption.keys.">[]EncryptionKey</a> |Defines the encryption keys generation and storage method.  | |
|`cipher` |string |Cipher to use for the encryption. Depends on the encryption provider. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
cipher: aes-xts-plain64
{{< /highlight >}}</details> |`aes-xts-plain64`<br />`xchacha12,aes-adiantum-plain64`<br />`xchacha20,aes-adiantum-plain64`<br /> |
|`keySize` |uint |Defines the encryption key length.  | |
|`blockSize` |uint64 |Defines the encryption sector size. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
blockSize: 4096
{{< /highlight >}}</details> | |
|`options` |[]string |Additional --perf parameters for the LUKS2 encryption. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
options:
    - no_read_workqueue
    - no_write_workqueue
{{< /highlight >}}</details> |`no_read_workqueue`<br />`no_write_workqueue`<br />`same_cpu_crypt`<br /> |




### keys[] {#UserVolumeConfig.encryption.keys.}

EncryptionKey represents configuration for disk encryption key.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`slot` |int |Key slot number for LUKS2 encryption.  | |
|`static` |<a href="#UserVolumeConfig.encryption.keys..static">EncryptionKeyStatic</a> |Key which value is stored in the configuration file.  | |
|`nodeID` |<a href="#UserVolumeConfig.encryption.keys..nodeID">EncryptionKeyNodeID</a> |Deterministically generated key from the node UUID and PartitionLabel.  | |
|`kms` |<a href="#UserVolumeConfig.encryption.keys..kms">EncryptionKeyKMS</a> |KMS managed encryption key.  | |
|`tpm` |<a href="#UserVolumeConfig.encryption.keys..tpm">EncryptionKeyTPM</a> |Enable TPM based disk encryption.  | |
|`lockToState` |bool |Lock the disk encryption key to the random salt stored in the STATE partition. This is useful to prevent the volume from being unlocked if STATE partition is compromised or replaced. It is recommended to use this option with TPM disk encryption for non-STATE volumes.  | |




#### static {#UserVolumeConfig.encryption.keys..static}

EncryptionKeyStatic represents throw away key type.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`passphrase` |string |Defines the static passphrase value.  | |






#### nodeID {#UserVolumeConfig.encryption.keys..nodeID}

EncryptionKeyNodeID represents deterministically generated key from the node UUID and PartitionLabel.









#### kms {#UserVolumeConfig.encryption.keys..kms}

EncryptionKeyKMS represents a key that is generated and then sealed/unsealed by the KMS server.



{{< highlight yaml >}}
encryption:
    keys:
        - kms:
            endpoint: https://192.168.88.21:4443 # KMS endpoint to Seal/Unseal the key.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`endpoint` |string |KMS endpoint to Seal/Unseal the key.  | |






#### tpm {#UserVolumeConfig.encryption.keys..tpm}

EncryptionKeyTPM represents a key that is generated and then sealed/unsealed by the TPM.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`options` |<a href="#UserVolumeConfig.encryption.keys..tpm.options">EncryptionKeyTPMOptions</a> |TPM options for key protection.  | |
|`checkSecurebootStatusOnEnroll` |bool |Check that Secureboot is enabled in the EFI firmware.<br>If Secureboot is not enabled, the enrollment of the key will fail.  | |




##### options {#UserVolumeConfig.encryption.keys..tpm.options}

EncryptionKeyTPMOptions represents the options for TPM-based key protection.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`pcrs` |[]int |List of PCRs to bind the key to. If not set, defaults to PCR 7, can be disabled by passing an empty list.  | |














