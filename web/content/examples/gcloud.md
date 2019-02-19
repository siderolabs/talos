 TL;DR:
 - Build a google cloud image with `make gcloud-image`
 - Upload the image with `gsutil cp /path/to/talos/build/gcloud/talos.tar.gz gs://gcloud-bucket-name`
 - Create a custom google cloud image with: 
 ```bash
 gcloud compute images create talos --source-uri=gs://gcloud-bucket-name/talos.tar.gz --guest-os-features=VIRTIO_SCSI_MULTIQUEUE
```
- Create instance in google cloud, making sure to create a `user-data` key in the "Metadata" section, with a value of your full talos node configuration. Further exploration needed to see if we can use the "Startup script" section instead.
