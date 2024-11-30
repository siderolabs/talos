---
description: PCIRebindConfig allows to configure PCI driver rebinds.
title: PCIRebindConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: PCIRebindConfig
name: ixgbe # Name of the config document.
vendorDeviceID: 0000:04:00.00 # PCI device vendor and device ID.
targetDriver: vfio-pci # Target driver to rebind the PCI device to.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of the config document.  | |
|`vendorDeviceID` |string |PCI device vendor and device ID.  | |
|`targetDriver` |string |Target driver to rebind the PCI device to.  | |






