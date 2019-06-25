---
title: "Google Cloud"
date: 2019-2-19
draft: false
menu:
  docs:
    parent: 'guides'
---

First, from a source checkout of the [Talos repository](https://www.github.com/talos-systems/talos/), create the Google Cloud compatible image:

```bash
make image-gcloud
```

Upload the image to Google Cloud with:

```bash
gsutil cp /path/to/talos/build/gcloud/talos.tar.gz gs://<gcloud bucket name>
```

Create a custom Google Cloud image with:

 ```bash
gcloud compute images create talos \
  --source-uri=gs://<gcloud bucket name>/talos.tar.gz \
  --guest-os-features=VIRTIO_SCSI_MULTIQUEUE
```

Create an instance in Google Cloud, making sure to create a `user-data` key in the "Metadata" section, with a value of your full Talos node configuration.

{{% note %}} Further exploration is needed to see if we can use the "Startup script" section instead. {{% /note %}}
