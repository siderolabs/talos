---
title: "Disk Encryption"
description: "Guide on using system disk encryption"
aliases:
  - ../../guides/disk-encryption
---

It is possible to enable encryption for system disks at the OS level.
Currently, only [STATE]({{< relref "../../learn-more/architecture/#file-system-partitions" >}}) and [EPHEMERAL]({{< relref "../../learn-more/architecture/#file-system-partitions" >}}) partitions can be encrypted.
STATE contains the most sensitive node data: secrets and certs.
The EPHEMERAL partition may contain sensitive workload data.
Data is encrypted using LUKS2, which is provided by the Linux kernel modules and `cryptsetup` utility.
The operating system will run additional setup steps when encryption is enabled.

If the disk encryption is enabled for the STATE partition, the system will:

- Save STATE encryption config as JSON in the META partition.
- Before mounting the STATE partition, load encryption configs either from the machine config or from the META partition.
  Note that the machine config is always preferred over the META one.
- Before mounting the STATE partition, format and encrypt it.
  This occurs only if the STATE partition is empty and has no filesystem.

If the disk encryption is enabled for the EPHEMERAL partition, the system will:

- Get the encryption config from the machine config.
- Before mounting the EPHEMERAL partition, encrypt and format it.
  This occurs only if the EPHEMERAL partition is empty and has no filesystem.

Talos Linux supports four encryption methods, which can be combined together for a single partition:

- `static` - encrypt with the static passphrase (weakest protection, for `STATE` partition encryption it means that the passphrase will be stored in the `META` partition).
- `nodeID` - encrypt with the key derived from the node UUID (weak, it is designed to protect against data being leaked or recovered from a drive that has been removed from a Talos Linux node).
- `kms` - encrypt using key sealed with network KMS (strong, but requires network access to decrypt the data.)
- `tpm` - encrypt with the key derived from the TPM (strong, when used with [SecureBoot]({{< relref "../install/bare-metal-platforms/secureboot" >}})).

> Note: `nodeID` encryption is not designed to protect against attacks where physical access to the machine, including the drive, is available.
> It uses the hardware characteristics of the machine in order to decrypt the data, so drives that have been removed, or recycled from a cloud environment or attached to a different virtual machine, will maintain their protection and encryption.
>
> Note: When using KMS encryption for `STATE` partition the network configuration can't be provided via the machine configuration, as KMS requires network connectivity before `STATE` partition is unlocked.
> Also custom CA certificates cannot be used for the KMS server, as these are stored in the `STATE` partition as well.

## Configuration

Disk encryption is disabled by default.
To enable disk encryption you should modify the machine configuration with the following options:

```yaml
machine:
  ...
  systemDiskEncryption:
    ephemeral:
      provider: luks2
      keys:
        - nodeID: {}
          slot: 0
    state:
      provider: luks2
      keys:
        - nodeID: {}
          slot: 0
```

### Encryption Keys

> Note: What the LUKS2 docs call "keys" are, in reality, a passphrase.
> When this passphrase is added, LUKS2 runs argon2 to create an actual key from that passphrase.

LUKS2 supports up to 32 encryption keys and it is possible to specify all of them in the machine configuration.
Talos always tries to sync the keys list defined in the machine config with the actual keys defined for the LUKS2 partition.
So if you update the keys list, keep at least one key that is not changed to be used for key management.

When you define a key you should specify the key kind and the `slot`:

```yaml
machine:
  ...
  state:
    keys:
      - nodeID: {} # key kind
        slot: 1

  ephemeral:
    keys:
      - static:
          passphrase: supersecret
        slot: 0
```

Take a note that key order does not play any role on which key slot is used.
Every key must always have a slot defined.

### Encryption Key Kinds

Talos supports two kinds of keys:

- `nodeID` which is generated using the node UUID and the partition label (note that if the node UUID is not really random it will fail the entropy check).
- `static` which you define right in the configuration.
- `kms` which is sealed with the network KMS.
- `tpm` which is sealed using the TPM and protected with SecureBoot.

> Note: Use static keys only if your STATE partition is encrypted and only for the EPHEMERAL partition.
> For the STATE partition it will be stored in the META partition, which is not encrypted.

### Key Rotation

In order to completely rotate keys, it is necessary to do `talosctl apply-config` a couple of times, since there is a need to always maintain a single working key while changing the other keys around it.

So, for example, first add a new key:

```yaml
machine:
  ...
  ephemeral:
    keys:
      - static:
          passphrase: oldkey
        slot: 0
      - static:
          passphrase: newkey
        slot: 1
  ...
```

Run:

```bash
talosctl apply-config -n <node> -f config.yaml
```

Then remove the old key:

```yaml
machine:
  ...
  ephemeral:
    keys:
      - static:
          passphrase: newkey
        slot: 1
  ...
```

Run:

```bash
talosctl apply-config -n <node> -f config.yaml
```

## Going from Unencrypted to Encrypted and Vice Versa

### Ephemeral Partition

There is no in-place encryption support for the partitions right now, so to avoid losing data only empty partitions can be encrypted.

As such, migration from unencrypted to encrypted needs some additional handling, especially around explicitly wiping partitions.

- `apply-config` should be called with `--mode=staged`.
- Partition should be wiped after `apply-config`, but before the reboot.

Edit your machine config and add the encryption configuration:

```bash
vim config.yaml
```

Apply the configuration with `--mode=staged`:

```bash
talosctl apply-config -f config.yaml -n <node ip> --mode=staged
```

Wipe the partition you're going to encrypt:

```bash
talosctl reset --system-labels-to-wipe EPHEMERAL -n <node ip> --reboot=true
```

That's it!
After you run the last command, the partition will be wiped and the node will reboot.
During the next boot the system will encrypt the partition.

### State Partition

Calling wipe against the STATE partition will make the node lose the config, so the previous flow is not going to work.

The flow should be to first wipe the STATE partition:

```bash
talosctl reset  --system-labels-to-wipe STATE -n <node ip> --reboot=true
```

Node will enter into maintenance mode, then run `apply-config` with `--insecure` flag:

```bash
talosctl apply-config --insecure -n <node ip> -f config.yaml
```

After installation is complete the node should encrypt the STATE partition.
