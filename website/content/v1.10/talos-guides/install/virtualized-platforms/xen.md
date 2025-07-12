---
title: "Xen"
aliases: 
  - ../../../virtualized-platforms/xen
---

Talos is known to work on Xen.
We don't yet have a documented guide specific to Xen; however, you can follow the [General Getting Started Guide]({{< relref "../../../introduction/getting-started" >}}).
If you run into any issues, our [community](https://slack.dev.talos-systems.io/) can probably help!

> Note: For Secure Boot, you can force setup mode with `varstore-sb-state <VM_UUID> setup` or `xe vm-set-uefi-mode mode=setup uuid=<VM_UUID>`.
> Don't forget to re-enable Secure Boot after the first boot.
