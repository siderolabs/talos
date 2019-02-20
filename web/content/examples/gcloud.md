---
title: "Google Cloud"
date: 2019-2-19
draft: false
weight: 20
menu:
  main:
    parent: 'examples'
    weight: 20
---

First, create the Google Cloud compatible image:

```bash
make image-gcloud
```

Upload the image with:

```bash
gsutil cp /path/to/talos/build/gcloud/talos.tar.gz gs://<gcloud bucket name>
```

Create a custom google cloud image with:

 ```bash
gcloud compute images create talos --source-uri=gs://<gcloud bucket name>/talos.tar.gz --guest-os-features=VIRTIO_SCSI_MULTIQUEUE
```

Create an instance in google cloud, making sure to create a `user-data` key in the "Metadata" section, with a value of your full talos node configuration.

{{% note %}} Further exploration is needed to see if we can use the "Startup script" section instead. {{% /note %}}
