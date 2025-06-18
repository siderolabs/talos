---
title: "Adding a Kernel Module"
description: "Create a system extension that includes kernel modules."
---

[System extensions]({{< relref "../talos-guides/configuration/system-extensions" >}}) in Talos provide ways to add files to the root filesystem and make it possible to run privileged containers as services.
But Talos still requires that kernel modules be signed with a trusted signing key to be loaded at run time.

To add a kernel module to Talos you will need to create a system extension that is built with the kernel and signed by the same signing key.
System extensions without kernel modules work without this requirement, but there are a couple extra steps needed when adding kernel modules.

## Create a package hello

Talos is built from the [pkgs](https://github.com/siderolabs/pkgs/) repo and the first step will be to add your custom package to that repo to be built with Talos.
You can look at other packages in that repository for examples of what should be included.

The only file required to create a package is a pkg.yaml file in a folder.
Let's create an example package to walk through each step.

Clone the repo and create a folder.

```bash
git clone https://github.com/siderolabs/pkgs
cd pkgs
mkdir my-module
```

Now add the package to the `.kres.yaml` file in the root of the repository.
We use this file for templating and generating Makefiles.
Put your out-of-tree kernel module below the comment for dependent packages.

```yaml
 spec:
   targets:
     ...
     # - kernel & dependent packages (out of tree kernel modules)
     #   kernel first, then packages in alphabetical order
     ...
     - my-module-pkg
     ...
```

Run the following command to generate a new Makefile.

```bash
make rekres
```

Now you have a `make` target to build your module and a directory to store your module configuration.
The next step is to create a `pkg.yaml` file to tell [`bldr`](https://github.com/siderolabs/bldr) how to create a container with the files you need.
The `bldr` tool has assumptions about directory structure and steps you can read about in the GitHub repo.

This example does not build a kernel module, but it can be used as a basis for your own packages.
Please also see existing pkg.yaml files in [the pkgs repo](https://github.com/siderolabs/pkgs)

```bash
name: my-module-pkg   # name of your package
variant: scratch      # base container for environment (e.g. alpine, scratch)
shell: /bin/sh        # shell to use to execute commands in steps
dependencies:         # other steps required before building package
  - stage: base
steps:                # steps needed to build package container
  - sources:          # download source files
      - url: https://example.com/source.tar.gz
        destination: my-module.tar.gz
        sha256: 1234abcd...
        sha521: abcd1234...
    prepare:          # create directories and untar
      - tar -xzf my-module.tar.gz --strip-components=1
    build:            # compiling software
      - make -j $(nproc)
    install:          # move compiled software to correct directory
      - make DESTDIR=/rootfs install
    test:             # validate software
      - fhs-validator /rootfs
finalize:             # copy directory structure from source to destination
  - from: /rootfs
    to: /
```

## Build the package and kernel

After you've created a pkg.yaml file you can test building your package with the make target you generated earlier.
Because Talos requires kernel modules to be signed with a signing key only available during the Talos kernel build process we need to build the kernel and package at the same time.

We also need a container registry available to store the built assets.
Follow the steps in [developing Talos]({{< relref "../advanced/developing-talos#prepare" >}}) to create a docker builder and run a local container registry before running this command.

```bash
make kernel my-module-pkg REGISTRY=127.0.0.1:5005 \
  PLATFORM=linux/amd64 \
  PUSH=true
```

If this is successful it should output two pieces of information we need to collect for the next steps.
We need to save the kernel and package images.
The output will look something like this:

```bash
=> => pushing manifest for 127.0.0.1:5005/user/kernel:v1.11.0-alpha.0...
...
=> => pushing manifest for 127.0.0.1:5005/user/my-module-pkg:v1.11.0-alpha.0...
```

For easier reference in this guide I will save these images as `$KERNEL_IMAGE` and `$PKG_IMAGE` variables.

## Create an extension

System extensions are the way to add software and files to a Talos Linux root filesystem.
Just like packages they are built as containers and then layered with Talos to create a bootable squashfs image.

The only unique thing about building a system extension with a kernel module is we need to build it against the kernel we just built in the previous step.
If we don't do this then our kernel module won't be signed and cannot be loaded at runtime.

The process is very similar to creating a package.
Start by cloning the extensions repo:

```bash
git clone https://github.com/siderolabs/extensions
cd extensions
mkdir my-module
```

Add your extension to the `.kres.yaml` file.

```yaml
---
kind: pkgfile.Build
spec:
  targets:
    ...
    - my-module
    ...
```

Then generate a new Makefile with additional target.

```bash
make rekres
```

Now create the `manifest.yaml` file for the metadata of your extension in the my-module folder.

```yaml
version: v1alpha1       # version of manifest.yaml
metadata:
  name: my-module
  version: 0.1
  author: me
  description: |
    An extension that adds a kernel module
  compatibility:
    talos:
      version: ">= v1.10.0"  # what version of Talos is supported
```

Create a pkg.yaml file in the my-module folder which works similarly to our pkg.yaml file for our package, but this time starts from the base image we built in the first step.
The local directory is mounted into the container at `/pkg` so we can copy files from that directory.

```yaml
name: my-module
variant: scratch
shell: /bin/sh
dependencies:
  - stage: base
  - image: "${PKG_IMAGE}"     # the image we built in the first step
steps:
  - install:
      - mkdir -p /rootfs/usr/lib/modules
      - cp -R /usr/lib/modules/* /rootfs/usr/lib/modules/
finalize:
  - from: /rootfs
    to: /rootfs
  - from: /pkg/manifest.yaml  # make sure you add the metadata file
    to: /
```

Lastly create a vars.yaml file to store a version variable in the my-module folder.
This isn't strictly required, but it is a convention used which will let the automated build work.

```bash
echo 'VERSION: "0.1"' > vars.yaml
```

## Build extension

You now have a complete extension config and can build it with the kernel from your previous pkg build.

```bash
make my-module REGISTRY=127.0.0.1:5005 \
  PLATFORM=linux/amd64 \
  PUSH=true
```

This will create a system extension image and push it to your local registry.
Copy the image that get's pushed and save it as `${EXTENSION_IMAGE}`.

```bash
 export EXTENSION_IMAGE='127.0.0.1:5005/jgarr/my-module:0.1@sha256:e8f3352...'
```

## Test the extension

Now we need to create installation media to boot Talos.
We will build and use [imager]({{< relref "../talos-guides/install/boot-assets#imager" >}}) to include our extension.

Clone the Talos repo.

```bash
git clone https://github.com/siderolabs/talos
cd talos
```

Build the installer, and remember to use the kernel image from the first step.

```bash
make installer-base imager PLATFORM=linux/amd64 \
  INSTALLER_ARCH=amd64 \
  REGISTRY=127.0.0.1:5005 \
  PKG_KERNEL=${KERNEL_IMAGE} \
  PUSH=true
```

This will create an imager image and push it to your local registry.
Export the image and save it as `$IMAGER_IMAGE`.

Create a installer image from your extension and the imager you just created with the following command.

```bash
make image-installer \
  REGISTRY=127.0.0.1:5005 \
  IMAGER_ARGS="--base-installer-image=${IMAGER_IMAGE} \
    --system-extension-image=${EXTENSION_IMAGE}"
```

We'll have a new container image tar file in the `_out/` folder of our repository.
Load and push the container image to a registry with [crane](https://github.com/google/go-containerregistry/blob/main/cmd/crane/doc/crane.md).
Make sure you replace `$REGISTRY`, `$USER`, and `$TAG` with the values you want.

```bash
crane push _out/installer-amd64.tar $REGISTRY/$USER/installer:$TAG
```

And if you don't have `crane`:

```bash
docker load -i _out/installer-amd64.tar
# note down sha256 or the image tag output from above command

docker tag $SHA256_OR_IMAGE_TAG $REGISTRY/$USER/installer:$TAG
docker push $REGISTRY/$USER/installer:$TAG
```

## Test the installer with fresh install

Now you can boot a machine from generic Talos installation media.
This is only used to get access to the API so we can apply a configuration that will use our installer image.
We'll assume this machine has an IP address of 192.168.100.100

Generate a configuration that uses your installer image.

```bash
talosctl gen config --install-image $REGISTRY/$USER/installer:$TAG \
    test https://192.168.100.100:6443     # cluster name and endpoint
```

Now create a configuration patch that loads your kernel module by name.
This should be the name of the `.ko` file you built in the package and put in the `/modules` directory.

```yaml
# my-module.yaml
machine:
  kernel:
    modules:
      - name: my-module
```

Apply the machine config and patch to your test machine.

```bash
talosctl apply -f controlplane.yaml -i -p '@my-module.yaml' -n 192.168.100.100
```

The machine will reboot as Talos is installed.
When the machine boots you should see logs that the module was loaded from dmesg.

```bash
192.168.100.100: kern: warning: my-module: loading out-of-tree module taints kernel.
192.168.100.100: kern:    info: Loading my-module driver module v0.1
```

## Test installer with existing machine

If you already have Talos running on a machine you can apply the installer during an upgrade to have the extension installed.

```bash
talosctl upgrade -i $REGISTRY/$USER/installer:$TAG
```

Make sure you still create a patch to load the kernel module and apply it to the machine.

```yaml
# my-module.yaml
machine:
  kernel:
    modules:
      - name: my-module
```

Apply the machine config and patch to your test machine.

```bash
talosctl apply -f controlplane.yaml -p '@my-module.yaml'
```
