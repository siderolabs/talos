---
title: "Scaleway"
description: "Creating a cluster via the CLI (scw) on scaleway.com."
aliases:
  - ../../../cloud-platforms/scaleway
---

Talos is known to work on scaleway.com; however, it is currently mostly undocumented.

> **Warning**: This guide is working with talos version >=1.10.6

The process to run a Talos node in Scaleway is as follows:

## Prerequisites

- Enable block storage on your Scaleway account (Scaleway will only allow snapshots from their block storage, not URLs)
- Configure the `scw` CLI to access your account (optional - you can use the console instead)
- Have `qemu-img` and `wget` installed for image conversion

## Image Preparation

1. **Download the image disk** of the Talos version you wish to run:

   ```bash
   wget "https://factory.talos.dev/image/376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba/v{{< release >}}/scaleway-amd64.raw.zst"
   ```

   > You can create your own brew on [Talos Factory](https://factory.talos.dev) if you need a custom image.

2. **Decompress and convert the image**:

   ```bash
   zstd --decompress scaleway-amd64.raw.zst
   qemu-img convert -O qcow2 scaleway-amd64.raw scaleway-amd64.qcow2
   ```

3. **Create S3 bucket** (if it doesn't exist):
   Go to Object Storage in the Scaleway console and create a new bucket.

4. **Upload to S3-compatible object storage**:
   Use the Scaleway console Object Storage interface to upload the QCOW2 file directly.

## Snapshot Creation

> Note: The following steps must be done via the CLI, as the Scaleway console does not support importing snapshots from object storage.

**Import snapshot from object storage** (repeat for each zone you need):

  ```bash
    scw block snapshot import-from-object-storage \
      name=talos-{{< release >}} \
      bucket=YOUR-BUCKET \
      key=scaleway-amd64.qcow2 \
      size=10GB \
      zone=fr-par-1
  ```

  Output will be similar to:

  ```text
    ID         e14679a6-23a9-4287-be43-356c5b512a66
    Name       scaleway-amd64.qcow2
    Size       10 GB
    ProjectID  5f3c2379-9596-48c6-811c-b6847fa1a31d
    CreatedAt  now
    UpdatedAt  now
    Status     creating
    Zone       fr-par-1
    Class      sbs
  ```

  > Keep the `ID` of the snapshot for the next step.

**Create image from snapshot** (optional - instances can be created directly from snapshots):

   ```bash
   scw instance image create \
     snapshot-id=SNAPSHOT-ID \
     arch=x86_64 \
     name=talos-{{< release >}} \
     zone=fr-par-1
   ```

  Output will be similar to:

  ```text
    ID                f6999f79-c92c-4e61-9ffc-5688c27f4943
    Name              talos-{{< release >}}
    Arch              x86_64
    CreationDate      now
    ModificationDate  now
    ExtraVolumes      0
    FromServer        -
    Organization      00000000-1876-4b3f-ae96-000000000000
    Public            false
    RootVolume        00000000-23a9-4287-be43-000000000000
    State             available
    Project           00000000-9596-48c6-811c-000000000000
    Zone              fr-par-1
  ```

## Instance Deployment

**Create instance** using the snapshot/image via GUI, CLI, or Infrastructure as Code tools.

## Notes

- The instance works correctly with Scaleway's reboot functionality
- `talosctl reset` performs the reset operation but don't start the maintenance mode automatically
