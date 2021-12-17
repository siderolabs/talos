---
title: Support Matrix
weight: 6
---

| Talos Version                                                                                                  | 0.15                               | 0.14                               |
|----------------------------------------------------------------------------------------------------------------|------------------------------------|------------------------------------|
| Release Date                                                                                                   | 2022-03-01, TBD                    | 2021-12-21 (0.14.0)                |
| End of Community Support                                                                                       | 0.16.0 release (2022-06-01, TBD)   | 0.15.0 release (2022-03-01, TBD)   |
| Enterprise Support                                                                                             | [offered by Sidero Labs Inc.](https://www.siderolabs.com/support/)      |
| Kubernetes                                                                                                     | 1.23, 1.22, 1.21                   | 1.23, 1.22, 1.21                   |
| Architecture                                                                                                   | amd64, arm64                                                            |
| **Platforms**                                                                                                  |                                    |                                    |
| - cloud                                                                                                        | AWS, GCP, Azure, Digital Ocean, Hetzner, OpenStack, Oracle Cloud, Scaleway, Vultr, Upcloud | AWS, GCP, Azure, Digital Ocean, Hetzner, OpenStack, Scaleway, Vultr, Upcloud  |
| - bare metal                                                                                                   | x86: BIOS, UEFI; arm64: UEFI; boot: ISO, PXE, disk image                |
| - virtualized                                                                                                  | VMware, Hyper-V, KVM, Proxmox, Xen                                      |
| - SBCs                                                                                                         | Raspberry Pi4, Banana Pi M64, Pine64, and other                         |
| - local                                                                                                        | Docker, QEMU                                                            |
| **Cluster API**                                                                                                |                                    |                                    |
| [CAPI Bootstrap Provider Talos](https://github.com/talos-systems/cluster-api-bootstrap-provider-talos)         | >= 0.4.3                           | >= 0.4.3                           |
| [CAPI Control Plane Provider Talos](https://github.com/talos-systems/cluster-api-control-plane-provider-talos) | >= 0.4.1                           | >= 0.4.1                           |
| [Sidero](https://www.sidero.dev/)                                                                              | >= 0.4.1                           | >= 0.4.1                           |
| **UI**                                                                                                         |                                    |                                    |
| [Theila](https://github.com/talos-systems/theila)                                                              | ✓                                  | ✓                                  |

## Platform Tiers

Tier 1: Automated tests, high-priority fixes.
Tier 2: Tested from time to time, medium-priority bugfixes.
Tier 3: Not tested by core Talos team, community tested.

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

* Hetzner
* nocloud
* Oracle Cloud
* Scaleway
* Vultr
* Upcloud
