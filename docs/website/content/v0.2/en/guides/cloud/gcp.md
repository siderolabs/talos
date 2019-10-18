---
title: 'GCP'
---

First, from a source checkout of the [Talos repository](https://www.github.com/talos-systems/talos/), create the GCP compatible image:

```bash
make image-gcloud
```

Upload the image to GCP with:

```bash
gsutil cp /path/to/talos/build/gcloud/talos.tar.gz gs://<gcloud bucket name>
```

Create a custom GCP image with:

```bash
gcloud compute images create talos \
 --source-uri=gs://<gcloud bucket name>/talos.tar.gz \
 --guest-os-features=VIRTIO_SCSI_MULTIQUEUE
```

Create an instance in GCP, making sure to create a `user-data` key in the "Metadata" section, with a value of your full Talos node configuration.

> Further exploration is needed to see if we can use the "Startup script" section instead.
