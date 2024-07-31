---
title: "Image Factory"
weight: 55
description: "Image Factory generates customized Talos Linux images based on configured schematics."
---

The Image Factory provides a way to download Talos Linux artifacts.
Artifacts can be generated with customizations defined by a "schematic".
A schematic can be applied to any of the versions of Talos Linux offered by the Image Factory to produce a "model".

The following assets are provided:

* ISO
* `kernel`, `initramfs`, and kernel command line
* UKI
* disk images in various formats (e.g. AWS, GCP, VMware, etc.)
* installer container images

The supported frontends are:

* HTTP
* PXE
* Container Registry

The official instance of Image Factory is available at https://factory.talos.dev.

See [Boot Assets]({{< relref "../talos-guides/install/boot-assets#image-factory" >}}) for an example of how to use the Image Factory to boot and upgrade Talos on different platforms.
Full API documentation for the Image Factory is available at [GitHub](https://github.com/siderolabs/image-factory#readme).

## Schematics

Schematics are YAML files that define customizations to be applied to a Talos Linux image.
Schematics can be applied to any of the versions of Talos Linux offered by the Image Factory to produce a "model", which is a Talos Linux image with the customizations applied.

Schematics are content-addressable, that is, the content of the schematic is used to generate a unique ID.
The schematic should be uploaded to the Image Factory first, and then the ID can be used to reference the schematic in a model.

Schematics can be generated using the [Image Factory UI](#ui), or using the Image Factory API:

```yaml
customization:
  extraKernelArgs: # optional
    - vga=791
  meta: # optional, allows to set initial Talos META
    - key: 0xa
      value: "{}"
  systemExtensions: # optional
    officialExtensions: # optional
      - siderolabs/gvisor
      - siderolabs/amd-ucode
overlay: # optional
  name: rpi_generic
  image: siderolabs/sbc-raspberry-pi
  options: # optional, any valid yaml, depends on the overlay implementation
    data: "mydata"
```

The "vanilla" schematic is:

```yaml
customization:
```

and has an ID of `376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba`.

The schematic can be applied by uploading it to the Image Factory:

```shell
curl -X POST --data-binary @schematic.yaml https://factory.talos.dev/schematics
```

As the schematic is content-addressable, the same schematic can be uploaded multiple times, and the Image Factory will return the same ID.

## Models

Models are Talos Linux images with customizations applied.
The inputs to generate a model are:

* schematic ID
* Talos Linux version
* model type (e.g. ISO, UKI, etc.)
* architecture (e.g. amd64, arm64)
* various model type specific options (e.g. disk image format, disk image size, etc.)

## Frontends

Image Factory provides several frontends to retrieve models:

* HTTP frontend to download models (e.g. download an ISO or a disk image)
* PXE frontend to boot bare-metal machines (PXE script references kernel/initramfs from HTTP frontend)
* Registry frontend to fetch customized `installer` images (for initial Talos Linux installation and upgrades)

The links to different models are available in the [Image Factory UI](#ui), and a full list of possible models is documented at [GitHub](https://github.com/siderolabs/image-factory#readme).

In this guide we will provide a list of examples:

* amd64 ISO (for Talos {{< release >}}, "vanilla" schematic) [https://factory.talos.dev/image/376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba/{{< release >}}/metal-amd64.iso](https://factory.talos.dev/image/376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba/{{< release >}}/metal-amd64.iso)
* arm64 AWS image (for Talos {{< release >}}, "vanilla" schematic) [https://factory.talos.dev/image/376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba/{{< release >}}/aws-arm64.raw.xz](https://factory.talos.dev/image/376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba/{{< release >}}/aws-arm64.raw.xz)
* amd64 PXE boot script (for Talos {{< release >}}, "vanilla" schematic) [https://pxe.factory.talos.dev/pxe/376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba/{{< release >}}/metal-amd64](https://pxe.factory.talos.dev/pxe/376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba/{{< release >}}/metal-amd64)
* Talos `installer` image (for Talos {{< release >}}, "vanilla" schematic, architecture is detected automatically): `factory.talos.dev/installer/376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba:{{< release >}}`

The `installer` image can be used to install Talos Linux on a bare-metal machine, or to upgrade an existing Talos Linux installation.
As the Talos version and schematic ID can be changed, via an upgrade process, the `installer` image can be used to upgrade to any version of Talos Linux, or replace a set of installed system extensions.

## UI

The Image Factory UI is available at https://factory.talos.dev.
The UI provides a way to list supported Talos Linux versions, list of system extensions available for each release, and a way to generate schematic based on the selected system extensions.

The UI operations are equivalent to API operations.

## Find Schematic ID from Talos Installation

Image Factory always appends "virtual" system extension with the version matching schematic ID used to generate the model.
So, for any running Talos Linux instance the schematic ID can be found by looking at the list of system extensions:

```shell
$ talosctl get extensions
NAMESPACE   TYPE              ID   VERSION   NAME       VERSION
runtime     ExtensionStatus   0    1         schematic  376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba
```

## Restrictions

Some models don't include every customization of the schematic:

* `installer` and `initramfs` images only support system extensions (kernel args and META are ignored)
* `kernel` assets don't depend on the schematic

Other models have full support for all customizations:

* any disk image format
* ISO, PXE boot script

When installing Talos Linux using ISO/PXE boot, Talos will be installed on the disk using the `installer` image, so the `installer` image in the machine configuration
should be using the same schematic as the ISO/PXE boot image.

Some system extensions are not available for all Talos Linux versions, so an attempt to generate a model with an unsupported system extension will fail.
List of supported Talos versions and supported system extensions for each version is available in the [Image Factory UI](#ui) and [API](https://github.com/siderolabs/image-factory#readme).

## Under the Hood

Image Factory is based on the Talos `imager` container which provides both the Talos base boot assets, and the ability to generate custom assets based on a configuration.
Image Factory manages a set of `imager` container images to acquire base Talos Linux boot assets (`kernel`, `initramfs`), a set of Talos Linux system extension images, and a set of schematics.
When a model is requested, Image Factory uses the `imager` container to generate the requested assets based on the schematic and the Talos Linux version.

## Security

Image Factory verifies signatures of all source container images fetched:

* `imager` container images (base boot assets)
* `extensions` system extensions catalogs
* `installer` contianer images (base installer layer)
* Talos Linux system extension images

Internally, Image Factory caches generated boot assets and signs all cached images using a private key.
Image Factory verifies the signature of the cached images before serving them to clients.

Image Factory signs generated `installer` images, and verifies the signature of the `installer` images before serving them to clients.

Image Factory does not provide a way to list all schematics, as schematics may contain sensitive information (e.g. private kernel boot arguments).
As the schematic ID is content-addressable, it is not possible to guess the ID of a schematic without knowing the content of the schematic.

## Running your own Image Factory

Image Factory can be deployed on-premises to provide in-house asset generation.

Image Factory requires following components:

* an OCI registry to store schematics (private)
* an OCI registry to store cached assets (private)
* an OCI registry to store `installer` images (should allow public read-only access)
* a container image signing key: ECDSA P-256 private key in PEM format

Image Factory is configured using command line flags, use `--help` to see a list of available flags.
Image Factory should be configured to use proper authentication to push to the OCI registries:

* by mounting proper credentials via `~/.docker/config.json`
* by supplying `GITHUB_TOKEN` (for `ghcr.io`)

Image Factory performs HTTP redirects to the public registry endpoint for `installer` images, so the public endpoint
should be available to Talos Linux machines to pull the `installer` images.
