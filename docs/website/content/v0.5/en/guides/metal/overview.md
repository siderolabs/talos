---
title: Deploying Talos on Bare Metal
---

In this section we will show how you can set up Talos in bare-metal environments.
Any tool using PXE booting can be used to deploy Talos.
We have some documented Talos working with several provisioning tools here:

- [Arges](arges) is a new project by Talos Systems designed to provide Talos users with a robust and reliable way to build and manage bare metal Talos-based Kubernetes clusters.
Arges uses Cluster-API for a consistent experience, and supports cloud platforms as well as bare metal.
- [Matchbox](matchbox) from Red Hat/CoreOS is a service that matches machines to profiles to PXE boot, and can be used to provision Talos clusters.

## Generic Information

### High level overview

Below is a image to visualize the process of bootstrapping nodes.

<img src="/images/metal-overview.png" width="950">

### Kernel Parameters

The following is a list of kernel parameters you will need to set:

- `talos.config` (required) the HTTP(S) URL at which the machine data can be found
- `talos.platform` (required) should be 'metal' for bare-metal installs

Talos also enforces some minimum requirements from the KSPP (kernel self-protection project).
The follow parameters are required:

- `page_poison=1`
- `slab_nomerge`
- `slub_debug=P`
- `pti=on`
