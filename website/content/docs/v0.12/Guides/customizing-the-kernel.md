---
title: "Customizing the Kernel"
description: ""
---

The installer image contains [`ONBUILD`](https://docs.docker.com/engine/reference/builder/#onbuild) instructions that handle the following:

- the decompression, and unpacking of the `initramfs.xz`
- the unsquashing of the rootfs
- the copying of new rootfs files
- the squashing of the new rootfs
- and the packing, and compression of the new `initramfs.xz`

When used as a base image, the installer will perform the above steps automatically with the requirement that a `customization` stage be defined in the `Dockerfile`.

Build and push your own kernel:

 ```sh
 git clone https://github.com/talos-systems/pkgs.git
 cd pkgs
 make kernel-menuconfig USERNAME=_your_github_user_name_

 docker login ghcr.io --username _your_github_user_name_
 make kernel USERNAME=_your_github_user_name_ PUSH=true
 ```

Using a multi-stage `Dockerfile` we can define the `customization` stage and build `FROM` the installer image:

```docker
FROM scratch AS customization
COPY --from=<custom kernel image> /lib/modules /lib/modules

FROM ghcr.io/talos-systems/installer:latest
COPY --from=<custom kernel image> /boot/vmlinuz /usr/install/${TARGETARCH}/vmlinuz
```

When building the image, the `customization` stage will automatically be copied into the rootfs.
The `customization` stage is not limited to a single `COPY` instruction.
In fact, you can do whatever you would like in this stage, but keep in mind that everything in `/` will be copied into the rootfs.

To build the image, run:

```bash
DOCKER_BUILDKIT=0 docker build --build-arg RM="/lib/modules" -t installer:kernel .
```

> Note: buildkit has a bug [#816](https://github.com/moby/buildkit/issues/816), to disable it use `DOCKER_BUILDKIT=0`

Now that we have a custom installer we can build Talos for the specific platform we wish to deploy to.
