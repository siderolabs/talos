---
title: "talosctl"
description: "Install Talos Linux CLI"
---

## Recommended

The client can be installed and updated via the [Homebrew package manager](https://brew.sh/) for macOS and Linux.
You will need to install `brew` and then you can install `talosctl` from the Sidero Labs tap.

```bash
brew install siderolabs/tap/talosctl
```

This will also keep your version of `talosctl` up to date with new releases.
This homebrew tap also has formulae for `omnictl` if you need to install that package.

> Note: Your `talosctl` version should match the version of Talos Linux you are running on a host.
> To install a specific version of `talosctl` with `brew` you can follow [this github issue](https://github.com/siderolabs/homebrew-tap/issues/75).

## Alternative install

You can automatically install the correct version of `talosctl` for your operating system and architecture with an installer script.
This script won't keep your version updated with releases and you will need to re-run the script to download a new version.

```bash
curl -sL https://talos.dev/install | sh
```

This script will work on macOS, Linux, and WSL on Windows.
It supports amd64 and arm64 architecture.

## Manual and Windows install

All versions can be manually downloaded from the [talos releases page](https://github.com/siderolabs/talos/releases/) including Linux, macOS, and Windows.

You will need to add the binary to a folder part of your executable `$PATH` to use it without providing the full path to the executable.

Updating the binary will be a manual process.
