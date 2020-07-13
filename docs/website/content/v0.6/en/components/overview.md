---
title: 'Components'
---

In this section we will discuss the various components of which Talos is comprised.

## Overview

| Component    | Description |
| ------------ | ----------- |
| [apid](apid) | When interacting with Talos, the gRPC API endpoint you're interact with directly is provided by `apid`. `apid` acts as the gateway for all component interactions and forwards the requests to `routerd`. |
| [containerd](containerd)  | An industry-standard container runtime with an emphasis on simplicity, robustness and portability. To learn more see the [containerd website](https://containerd.io). |
| [machined](machined) | Talos replacement for the traditional Linux init-process. Specially designed to run Kubernetes and does not allow starting arbitrary user services. |
| [networkd](networkd) | Handles all of the host level network configuration. Configuration is defined under the `networking` key |
| [timed](timed) | Handles the host time synchronization by acting as a NTP-client. |
| [kernel](kernel) | The Linux kernel included with Talos is configured according to the recommendations outlined in the  [Kernel Self Protection Project](http://kernsec.org/wiki/index.php/Kernel_Self_Protection_Project). |
| [routerd](routerd) | Responsible for routing an incoming API request from `apid` to the appropriate backend (e.g. `osd`, `machined` and `timed`). |
| [trustd](trustd) | To run and operate a Kubernetes cluster a certain level of trust is required. Based on the concept of a 'Root of Trust', `trustd` is a simple daemon responsible for establishing trust within the system. |
| [udevd](udevd) | Implementation of `eudev` into `machined`. `eudev` is Gentoo's fork of udev, systemd's device file manager for the Linux kernel. It manages device nodes in /dev and handles all user space actions when adding or removing devices. To learn more see the [Gentoo Wiki](https://wiki.gentoo.org/wiki/Eudev). |
