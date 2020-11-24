---
title: "Customizing the Root Filesystem"
description: ""
---

The installer image contains [`ONBUILD`](https://docs.docker.com/engine/reference/builder/#onbuild) instructions that handle the following:

- the decompression, and unpacking of the `initramfs.xz`
- the unsquashing of the rootfs
- the copying of new rootfs files
- the squashing of the new rootfs
- and the packing, and compression of the new `initramfs.xz`

When used as a base image, the installer will perform the above steps automatically with the requirement that a `customization` stage be defined in the `Dockerfile`.

For example, say we have an image that contains the contents of a library we wish to add to the Talos rootfs.
We need to define a stage with the name `customization`:

```docker
FROM scratch AS customization
COPY --from=<name|index> <src> <dest>
```

Using a multi-stage `Dockerfile` we can define the `customization` stage and build `FROM` the installer image:

```docker
FROM scratch AS customization
COPY --from=<name|index> <src> <dest>

FROM ghcr.io/talos-systems/installer:latest
```

When building the image, the `customization` stage will automatically be copied into the rootfs.
The `customization` stage is not limited to a single `COPY` instruction.
In fact, you can do whatever you would like in this stage, but keep in mind that everything in `/` will be copied into the rootfs.

> Note: `<dest>` is the path relative to the rootfs that you wish to place the contents of `<src>`.

To build the image, run:

```bash
docker build --squash -t <organization>/installer:latest .
```

In the case that you need to perform some cleanup _before_ adding additional files to the rootfs, you can specify the `RM` [build-time variable](https://docs.docker.com/engine/reference/commandline/build/#set-build-time-variables---build-arg):

```bash
docker build --squash --build-arg RM="[<path> ...]" -t <organization>/installer:latest .
```

This will perform a `rm -rf` on the specified paths relative to the rootfs.

> Note: `RM` must be a whitespace delimited list.

The resulting image can be used to:

- generate an image for any of the supported providers
- perform bare-metall installs
- perform upgrades

We will step through common customizations in the remainder of this section.
