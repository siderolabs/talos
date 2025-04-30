---
description: PCIDriverRebindConfig allows to configure PCI driver rebinds.
title: PCIDriverRebindConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: PCIDriverRebindConfig
name: 0000:04:00.00 # PCI device id
targetDriver: vfio-pci # Target driver to rebind the PCI device to.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |PCI device id  | |
|`targetDriver` |string |Target driver to rebind the PCI device to.  | |






