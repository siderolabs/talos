---
title: "Image Cache"
description: "How to enable and configure Talos image cache feature."
---

Talos Image Cache feature allows to provide container images to the nodes without the need to pull them from the Internet.
This feature is useful in environments with limited or no Internet access.

Image Cache is local to the machine, and automatically managed by Talos if enabled.

## Preparing Image Cache

First, build a list of image references that need to be cached.
The `talosctl images default` might be used as a starting point, but it should be customized to include additional images (e.g. custom CNI, workload images, etc.)

```bash
talosctl images default > images.txt
cat extra-images.txt >> images.txt
```

Next, prepare an OCI image which contains all cached images:

```bash
cat images.txt | talosctl images cache-create --image-cache-path ./image-cache.oci --images=-
```

> Note: The `cache-create` supports a `--layer-cache` flag to additionally cache the pulled images layers on the filesystem.
> This is useful to speed up repeated calls for `cache-create` with the same images.

The OCI image cache directory might be used directly (`./image-cache.oci`) or pushed itself to a container registry of your choice (e.g. with `crane push`).

Example of pushing the OCI image cache directory to a container registry:

```bash
crane push ./image-cache.oci my.registry/image-cache:my-cache
```

## Building Boot Assets

The image cache is provided to Talos via the boot assets.
There are two supported boot asset types for the Image Cache: ISO and disk image.

### ISO

In case of ISO, the image cache is bundled with a Talos ISO image, it will be available for the initial install and (if configured) copied to the
disk during the installation process.

The ISO image can built with the [imager]({{< relref "../install/boot-assets#imager" >}}) by passing an additional `--image-cache` flag:

```bash
mkdir -p _out/
docker run --rm -t -v $PWD/_out:/secureboot:ro -v $PWD/_out:/out -v $PWD/image-cache.oci:/image-cache.oci:ro -v /dev:/dev --privileged ghcr.io/siderolabs/imager:{{< release >}} iso --image-cache /image-cache.oci
```

> Note: If the image cache was pushed to a container registry, the `--image-cache` flag should point to the image reference.
> SecureBoot ISO is supported as well.

The ISO image can be utilized in the following ways (which allows both booting Talos and using the image cache):

* Using a physical or virtual CD/DVD drive.
* Copying the ISO image to a USB drive using `dd`.
* Copying the contents of the ISO image to a FAT-formatted USB drive with a volume label that starts with `TALOS_`, such as `TALOS_1` (only for UEFI systems).

> Note: Third-party boot loaders, such as Ventoy, are not supported as Talos will not be able to access the image cache.

### Disk Image

In case of disk image, the image cache is included in the disk image itself, and on boot it would be used immediately by the Talos.

The disk image can be built with the [imager]({{< relref "../install/boot-assets#imager" >}}) by passing an additional `--image-cache` flag:

```bash
mkdir -p _out/
docker run --rm -t -v $PWD/_out:/secureboot:ro -v $PWD/_out:/out -v $PWD/image-cache.oci:/image-cache.oci:ro -v /dev:/dev --privileged ghcr.io/siderolabs/imager:{{< release >}} metal --image-cache /image-cache.oci
```

> Note: If the image cache was pushed to a container registry, the `--image-cache` flag should point to the image reference.

For a disk image, the `IMAGECACHE` partition will use all available space on the disk image (excluding the mandatory boot partitions).
Therefore, you may need to adjust the disk image size using the `--image-disk-size` flag to ensure the `IMAGECACHE` partition is large enough to accommodate the image cache contents, for example, `--image-disk-size=4GiB`.

Upon boot, Talos will expand the disk image to utilize the full disk size.

## Configuration

The image cache feature (for security reasons) should be explicitly enabled in the Talos configuration:

```yaml
machine:
  features:
    imageCache:
      localEnabled: true
```

Once enabled, Talos Linux will automatically look for the image cache contents either on the disk or in the ISO image.

If the image cache is bundled with the ISO, the disk volume size for the image cache should be configured to copy the image cache to the disk during the installation process:

```yaml
apiVersion: v1alpha1
kind: VolumeConfig
name: IMAGECACHE
provisioning:
  diskSelector:
    match: 'system_disk'
  minSize: 2GB
  maxSize: 2GB
```

The default settings for the `IMAGECACHE` volume are as follows (note that a configuration should still be provided to enable the image cache volume provisioning):

* `minSize: 500MB`
* `maxSize: 1GB`
* `diskSelector: match: system_disk`

In this example, image cache volume is provisioned on the system disk with a fixed size of 2GB.
The size of the volume should be adjusted to fit the image cache.
You can see the size of your cache by looking at the size of the image-cache.oci folder with `du -sh image-cache.oci`.

If the disk image is used, the `IMAGECACHE` volume doesn't need to be configured, as the image cache volume is already present in the disk image.

See [disk management]({{< relref "./disk-management#machine-configuration" >}}) for more information on volume configuration.

## Troubleshooting

When the image cache is enabled, Talos will block on boot waiting for the image cache to be available:

```text
task install (1/1): waiting for the image cache
```

After the initial install from an ISO, the image cache will be copied to the disk and will be available for the subsequent boots:

```text
task install (1/1): waiting for the image cache copy
copying image cache {"component": "controller-runtime", "controller": "cri.ImageCacheConfigController", "source": "/system/imagecache/iso/imagecache", "target": "/system/imagecache/disk"}
image cache copied {"component": "controller-runtime", "controller": "cri.ImageCacheConfigController", "size": "414 MiB"}
```

The current status of the image cache can be checked via the `ImageCacheConfig` resource:

```yaml
# talosctl get imagecacheconfig -o yaml
spec:
  status: ready
  copyStatus: ready
  roots:
    - /system/imagecache/disk
    - /system/imagecache/iso/imagecache
```

The `status` field indicates the readiness of the image cache, and the `copyStatus` field indicates the readiness of the image cache copy.
The `roots` field contains the paths to the image cache contents, in this example both on-disk and ISO caches are available.
Image cache roots are used in order they are listed.

You can get logs from the registry to see if images are being "hit" (a.k.a. cached) or "missed" (a.k.a. pulled from upstream).

```bash
talosctl logs registryd
```
