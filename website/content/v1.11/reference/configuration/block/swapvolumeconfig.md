---
description: |
    SwapVolumeConfig is a disk swap volume configuration document.
    Swap volume is automatically allocated as a partition on the specified disk
    and activated as swap, removing a swap volume deactivates swap.
    The partition label is automatically generated as `s-<name>`.
title: SwapVolumeConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: SwapVolumeConfig
name: swap1 # Name of the volume.
# The provisioning describes how the volume is provisioned.
provisioning:
    # The disk selector expression.
    diskSelector:
        match: disk.transport == "nvme" # The Common Expression Language (CEL) expression to match the disk.
    minSize: 3GiB # The minimum size of the volume.
    maxSize: 4GiB # The maximum size of the volume, if not specified the volume can grow to the size of the
# The encryption describes how the volume is encrypted.
encryption:
    provider: luks2 # Encryption provider to use for the encryption.
    # Defines the encryption keys generation and storage method.
    keys:
        - slot: 0 # Key slot number for LUKS2 encryption.
          # Key which value is stored in the configuration file.
          static:
            passphrase: swapsecret # Defines the static passphrase value.

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
|`provisioning` |<a href="#SwapVolumeConfig.provisioning">ProvisioningSpec</a> |The provisioning describes how the volume is provisioned.  | |
|`encryption` |<a href="#SwapVolumeConfig.encryption">EncryptionSpec</a> |The encryption describes how the volume is encrypted.  | |




## provisioning {#SwapVolumeConfig.provisioning}

ProvisioningSpec describes how the volume is provisioned.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`diskSelector` |<a href="#SwapVolumeConfig.provisioning.diskSelector">DiskSelector</a> |The disk selector expression.  | |
|`grow` |bool |Should the volume grow to the size of the disk (if possible).  | |
|`minSize` |ByteSize |The minimum size of the volume.<br><br>Size is specified in bytes, but can be expressed in human readable format, e.g. 100MB. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
minSize: 2.5GiB
{{< /highlight >}}</details> | |
|`maxSize` |ByteSize |The maximum size of the volume, if not specified the volume can grow to the size of the<br>disk.<br><br>Size is specified in bytes, but can be expressed in human readable format, e.g. 100MB. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
maxSize: 50GiB
{{< /highlight >}}</details> | |




### diskSelector {#SwapVolumeConfig.provisioning.diskSelector}

DiskSelector selects a disk for the volume.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`match` |Expression |The Common Expression Language (CEL) expression to match the disk. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
match: disk.size > 120u * GB && disk.size < 1u * TB
{{< /highlight >}}{{< highlight yaml >}}
match: disk.transport == "sata" && !disk.rotational && !system_disk
{{< /highlight >}}</details> | |








## encryption {#SwapVolumeConfig.encryption}

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
|`keys` |<a href="#SwapVolumeConfig.encryption.keys.">[]EncryptionKey</a> |Defines the encryption keys generation and storage method.  | |
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




### keys[] {#SwapVolumeConfig.encryption.keys.}

EncryptionKey represents configuration for disk encryption key.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`slot` |int |Key slot number for LUKS2 encryption.  | |
|`static` |<a href="#SwapVolumeConfig.encryption.keys..static">EncryptionKeyStatic</a> |Key which value is stored in the configuration file.  | |
|`nodeID` |<a href="#SwapVolumeConfig.encryption.keys..nodeID">EncryptionKeyNodeID</a> |Deterministically generated key from the node UUID and PartitionLabel.  | |
|`kms` |<a href="#SwapVolumeConfig.encryption.keys..kms">EncryptionKeyKMS</a> |KMS managed encryption key.  | |
|`tpm` |<a href="#SwapVolumeConfig.encryption.keys..tpm">EncryptionKeyTPM</a> |Enable TPM based disk encryption.  | |




#### static {#SwapVolumeConfig.encryption.keys..static}

EncryptionKeyStatic represents throw away key type.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`passphrase` |string |Defines the static passphrase value.  | |






#### nodeID {#SwapVolumeConfig.encryption.keys..nodeID}

EncryptionKeyNodeID represents deterministically generated key from the node UUID and PartitionLabel.









#### kms {#SwapVolumeConfig.encryption.keys..kms}

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






#### tpm {#SwapVolumeConfig.encryption.keys..tpm}

EncryptionKeyTPM represents a key that is generated and then sealed/unsealed by the TPM.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`checkSecurebootStatusOnEnroll` |bool |Check that Secureboot is enabled in the EFI firmware.<br>If Secureboot is not enabled, the enrollment of the key will fail. As the TPM key is anyways bound to the value of PCR 7, changing Secureboot status or configuration after the initial enrollment will make the key unusable.  | |












