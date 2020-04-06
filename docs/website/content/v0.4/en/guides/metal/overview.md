---
title: Deploying Talos on Bare Metal
---

In this section we will show how you can setup Talos in bare-metal environments.

## Kernel Parameters

The following is a list of kernel parameters you will need to set:

- `talos.config` (required) the HTTP(S) URL at which the machine data can be found
- `talos.platform` (required) should be 'metal' for bare-metal installs

Talos also enforces some minimum requirements from the KSPP (kernel self-protection project).
The follow parameters are required:

- `page_poison=1`
- `slab_nomerge`
- `slub_debug=P`
- `pti=on`

You may experience long boot times due to low entropy.
In newer versions of Linux, the initialization of the Cryptographically (Secure) Random Number Generator (CRNG) will block until it is properly seeded from the entropy pool at boot time.
To work around this, add `random.trust_cpu=on` to the list of kernel arguments.
