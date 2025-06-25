---
title: Support Matrix
weight: 60
description: "Table of supported Talos Linux versions and respective platforms."
---

| Talos Version                                                                                               | 1.10                                                                                                                                                                                                        | 1.9                                                                                                                                                                                                                                |
| ----------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | ---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Release Date                                                                                                | 2025-04-30                                                                                                                                                                                                  | 2024-12-17 (1.9.0)                                                                                                                                                                                                                 |
| End of Community Support                                                                                    | 1.11.0 release (2025-08-15, TBD)                                                                                                                                                                            | 1.10.0 release (2025-04-30)                                                                                                                                                                                                        |
| Enterprise Support                                                                                          | [offered by Sidero Labs Inc.](https://www.siderolabs.com/support/)                                                                                                                                          | [offered by Sidero Labs Inc.](https://www.siderolabs.com/support/)                                                                                                                                                                 |
| Kubernetes                                                                                                  | 1.33, 1.32, 1.31, 1.30, 1.29, 1.28                                                                                                                                                                          | 1.32, 1.31, 1.30, 1.29, 1.28, 1.27                                                                                                                                                                                                 |
| NVIDIA Drivers                                                                                              | 570.x.x (PRODUCTION), 535.x.x (LTS)                                                                                                                                                                         | 550.x.x (PRODUCTION), 535.x.x (LTS)                                                                                                                                                                                                |
| Architecture                                                                                                | amd64, arm64                                                                                                                                                                                                | amd64, arm64                                                                                                                                                                                                                       |
| **Platforms**                                                                                               |                                                                                                                                                                                                             |                                                                                                                                                                                                                                    |
| - cloud                                                                                                     | Akamai, AWS, GCP, Azure, CloudStack, Digital Ocean, Exoscale, Hetzner, OpenNebula, OpenStack, Oracle Cloud, Scaleway, Vultr, Upcloud                                                                        | Akamai, AWS, GCP, Azure, CloudStack, Digital Ocean, Exoscale, Hetzner, OpenNebula, OpenStack, Oracle Cloud, Scaleway, Vultr, Upcloud                                                                                               |
| - bare metal                                                                                                | x86: BIOS, UEFI, SecureBoot; arm64: UEFI, SecureBoot; boot: ISO, PXE, disk image                                                                                                                            | x86: BIOS, UEFI; arm64: UEFI; boot: ISO, PXE, disk image                                                                                                                                                                           |
| - virtualized                                                                                               | VMware, Hyper-V, KVM, Proxmox, Xen                                                                                                                                                                          | VMware, Hyper-V, KVM, Proxmox, Xen                                                                                                                                                                                                 |
| - SBCs                                                                                                      | Banana Pi M64, Jetson Nano, Libre Computer Board ALL-H3-CC, Nano Pi R4S, Pine64, Pine64 Rock64, Radxa ROCK Pi 4C, Radxa ROCK 4C+, Radxa ROCK 5B, Raspberry Pi 4B, Raspberry Pi Compute Module 4, Turing RK1, Orange Pi 5 | Banana Pi M64, Jetson Nano, Libre Computer Board ALL-H3-CC, Nano Pi R4S, Orange Pi R1 Plus LTS, Pine64, Pine64 Rock64, Radxa ROCK Pi 4C, Radxa ROCK 4C+, Radxa ROCK 5B, Raspberry Pi 4B, Raspberry Pi Compute Module 4, Turing RK1, Orange Pi 5 |
| - local                                                                                                     | Docker, QEMU                                                                                                                                                                                                | Docker, QEMU                                                                                                                                                                                                                       |
| **Omni**                                                                                                    |                                                                                                                                                                                                             |                                                                                                                                                                                                                                    |
| [Omni](https://github.com/siderolabs/omni)                                                                  | >= 0.50.0                                                                                                                                                                                                   | >= 0.45.0                                                                                                                                                                                                                          |
| **Cluster API**                                                                                             |                                                                                                                                                                                                             |                                                                                                                                                                                                                                    |
| [CAPI Bootstrap Provider Talos](https://github.com/siderolabs/cluster-api-bootstrap-provider-talos)         | >= 0.6.8                                                                                                                                                                                                    | >= 0.6.7                                                                                                                                                                                                                           |
| [CAPI Control Plane Provider Talos](https://github.com/siderolabs/cluster-api-control-plane-provider-talos) | >= 0.5.9                                                                                                                                                                                                    | >= 0.5.8                                                                                                                                                                                                                           |
| [Sidero](https://www.sidero.dev/)                                                                           | >= 0.6.6                                                                                                                                                                                                    | >= 0.6.5                                                                                                                                                                                                                           |

## Platform Tiers

* Tier 1: Automated tests, high-priority fixes.
* Tier 2: Tested from time to time, medium-priority bugfixes.
* Tier 3: Not tested by core Talos team, community tested.

### Tier 1

* Metal
* AWS
* GCP

### Tier 2

* Azure
* Digital Ocean
* OpenStack
* VMWare

### Tier 3

* Akamai
* CloudStack
* Exoscale
* Hetzner
* nocloud
* OpenNebula
* Oracle Cloud
* Scaleway
* Vultr
* Upcloud
