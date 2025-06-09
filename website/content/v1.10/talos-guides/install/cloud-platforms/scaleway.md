---
title: "Scaleway"
description: "Creating a cluster via the CLI (scw) on scaleway.com."
aliases:
  - ../../../cloud-platforms/scaleway
---

Talos is known to work on scaleway.com; however, it is currently mostly undocumented.

The process to run a Talos node in Scaleway is as follows :

- Enable block storage on your Scaleway account (Scaleway will only allow snapshots from their block storage, not urls).
- Download the `qcow` image of the Talos version you wish to run.
- Upload it to your block storage using an s3 client or the web interface.
- Configure the `scw` CLI to be able to access your account.
- Create a snapshot from the image :
  - `scw block snapshot import-from-object-storage name=talos-{{< release >}} bucket=YOUR-BUCKET key=talos.qcow2 size=10GB` (this is assuming that you uploaded to a bucket called `YOUR-BUCKET` and a filename of `talos.qcow2`, to be called `talos-{{< release >}}`.
- Create your instance using the snapshot using the GUI/CLI/OpenTofu etc.
