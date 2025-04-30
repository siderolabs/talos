---
title: "Boot Loader"
description: "Overview of the Talos boot process and boot loader configuration."
---

Talos uses two boot loaders based on system architecture and configuration:

* GRUB
* `systemd-boot`

GRUB is used for legacy BIOS systems on x86_64, while `systemd-boot` is used for UEFI systems on x86_64 and arm64.

> Note: When upgrading from earlier Talos versions, the existing bootloader is retained.
> Prior to Talos 1.10, GRUB was the default for all systems except SecureBoot images, which used `systemd-boot`.

To check the current bootloader:

```shell
$ talosctl get securitystate -o yaml
spec:
    # ...
    bootedWithUKI: true # Indicates systemd-boot is in use
```

## `systemd-boot`

`systemd-boot` is the default bootloader for UEFI systems on x86_64 and arm64.
It is a lightweight boot manager from the `systemd` project, designed for simplicity and speed.

Talos boots via UKI (Unified Kernel Image), a single binary containing the kernel, initramfs, and kernel command line arguments.
The UKI may include multiple profiles with different kernel arguments, such as regular boot and wiping mode.
These profiles are displayed in the `systemd-boot` menu.

Partition layout for `systemd-boot`:

* `EFI`: Contains the `systemd-boot` bootloader and Talos UKIs.

On UEFI systems, the `EFI` partition is automatically detected by the system firmware.
Since UKIs are EFI binaries, they can also be booted directly from the EFI shell or firmware boot menu, including HTTP boot.

With `systemd-boot`, the `.machine.install.extraKernelArgs` field in the machine configuration is ignored, as kernel arguments are embedded in the UKI and cannot be modified without upgrading the UKI.

## GRUB

GRUB boots Talos using `vmlinuz`, `initramfs`, and kernel arguments stored in its configuration file.

> Note: GRUB was previously used for UEFI systems but is no longer used for new installations starting with Talos 1.10.

Partition layout for GRUB:

* MBR (Master Boot Record): Contains the initial boot code.
* `BIOS`: Contains the GRUB bootloader.
* `EFI`: Contains the GRUB bootloader for UEFI systems (only for upgrades from earlier Talos versions).
* `BOOT`: Contains the GRUB configuration file and Talos boot assets (`vmlinuz`, `initramfs`).

With GRUB, kernel arguments are stored in the GRUB configuration file.
The `.machine.install.extraKernelArgs` field in the machine configuration can be used to modify these arguments, followed by an upgrade.
