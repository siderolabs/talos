---
title: Support Matrix
weight: 6
---

| Talos Version                                                                                                  | 0.12                               | 0.11                               |
|----------------------------------------------------------------------------------------------------------------|------------------------------------|------------------------------------|
| Release Date                                                                                                   | 2021-08-30 (TBD)                   | 2021-07-08 (0.11.0)                |
| End of Community Support                                                                                       | 0.13.0 release (2021-10-15, TBD)   | 2021-09-15                         |
| Enterprise Support                                                                                             | [offered by Talos Systems Inc.](https://www.talos-systems.com/support/) |
| Kubernetes                                                                                                     | 1.22, 1.21, 1.20                   | 1.21, 1.20, 1.19                   |
| Architecture                                                                                                   | amd64, arm64                                                            |
| **Platforms**                                                                                                  |                                    |                                    |
| - cloud                                                                                                        | AWS, GCP, Azure, Digital Ocean, OpenStack                               |
| - bare metal                                                                                                   | x86: BIOS, UEFI; arm64: UEFI; boot: ISO, PXE, disk image                |
| - virtualized                                                                                                  | VMware, Hyper-V, KVM, Proxmox, Xen                                      |
| - SBCs                                                                                                         | Raspberry Pi4, Banana Pi M64, Pine64, and other                         |
| - local                                                                                                        | Docker, QEMU                                                            |
| **Cluster API**                                                                                                |                                    |                                    |
| [CAPI Bootstrap Provider Talos](https://github.com/talos-systems/cluster-api-bootstrap-provider-talos)         | >= 0.2.0                           | >= 0.2.0                           |
| [CAPI Control Plane Provider Talos](https://github.com/talos-systems/cluster-api-control-plane-provider-talos) | >= 0.1.1                           | >= 0.1.1                           |
| [Sidero](https://www.sidero.dev/)                                                                              | >= 0.3.0                           | >= 0.3.0                           |
| **UI**                                                                                                         |                                    |                                    |
| [Theila](https://github.com/talos-systems/theila)                                                              | ✓                                  | ✓                                  |

## Platform Tiers

Tier 1: Automated tests, high-priority fixes.
Tier 2: Tested from time to time, medium-priority bugfixes.
Tier 3: Not tested by core Talos team, community tested.

| Platform | Tier |
|-----------------|
| AWS      |   1  |
| GCP      |   1  |
| Metal    |   1  |
| Azure    |   2  |
| VMware   |   2  |

