---
title: "Adding a proprietary kernel module to Talos Linux"
description: ""
---

1. Patching and building the kernel image
    1. Clone the `pkgs` repository from Github and check out the revision corresponding to your version of Talos Linux

        ```bash
        git clone https://github.com/talos-systems/pkgs pkgs && cd pkgs
        git checkout v0.8.0
        ```

    2. Clone the Linux kernel and check out the revision that pkgs uses (this can be found in `kernel/kernel-prepare/pkg.yaml` and it will be something like the following: `https://cdn.kernel.org/pub/linux/kernel/v5.x/linux-x.xx.x.tar.xz`)

        ```bash
        git clone https://git.kernel.org/pub/scm/linux/kernel/git/torvalds/linux.git && cd linux
        git checkout v5.15
        ```

    3. Your module will need to be converted to be in-tree.
       The steps for this are different depending on the complexity of the module to port, but generally it would involve moving the module source code into the `drivers` tree and creating a new Makefile and Kconfig.
    4. Stage your changes in Git with `git add -A`.
    5. Run `git diff --cached --no-prefix > foobar.patch` to generate a patch from your changes.
    6. Copy this patch to `kernel/kernel/patches` in the `pkgs` repo.
    7. Add a `patch` line in the `prepare` segment of `kernel/kernel/pkg.yaml`:

        ```bash
        patch -p0 < /pkg/patches/foobar.patch
        ```

    8. Build the kernel image.
       Make sure you are logged in to `ghcr.io` before running this command, and you can change or omit `PLATFORM` depending on what you want to target.

        ```bash
        make kernel PLATFORM=linux/amd64 USERNAME=your-username PUSH=true
        ```

    9. Make a note of the image name the `make` command outputs.
2. Building the installer image
    1. Copy the following into a new `Dockerfile`:

        ```dockerfile
        FROM scratch AS customization
        COPY --from=ghcr.io/your-username/kernel:<kernel version> /lib/modules /lib/modules

        FROM ghcr.io/talos-systems/installer:<talos version>
        COPY --from=ghcr.io/your-username/kernel:<kernel version> /boot/vmlinuz /usr/install/${TARGETARCH}/vmlinuz
        ```

    2. Run to build and push the installer:

        ```bash
        INSTALLER_VERSION=<talos version>
        IMAGE_NAME="ghcr.io/your-username/talos-installer:$INSTALLER_VERSION"
        DOCKER_BUILDKIT=0 docker build --build-arg RM="/lib/modules" -t "$IMAGE_NAME" . && docker push "$IMAGE_NAME"
        ```

3. Deploying to your cluster

    ```bash
    talosctl upgrade --image ghcr.io/your-username/talos-installer:<talos version> --preserve=true
    ```
