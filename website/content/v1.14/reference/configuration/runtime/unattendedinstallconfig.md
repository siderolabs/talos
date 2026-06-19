---
description: UnattendedInstallConfig is an UnattendedInstallConfig config document.
title: UnattendedInstallConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: UnattendedInstallConfig
# The installer describes the source of the installation.
installer:
    image: factory.talos.dev/metal-installer/376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba:latest # Allows for supplying the image used to perform the installation.
# The provisioning describes how the installation disk should be provisioned.
provisioning:
    # Matches disks to initialize as physical volumes.
    diskSelector:
        match: disk.dev_path == "/dev/sda" # CEL expression matching a disk.
    wipe: true # Indicates if the installation disk should be wiped at installation time.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`reboot` |bool |Reboot is a flag to indicate if the system should reboot after installation.<br>If not set, Talos will reboot only if the installer.image is set.<br>  | |
|`installer` |<a href="#UnattendedInstallConfig.installer">InstallerSpec</a> |The installer describes the source of the installation. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
installer:
    image: factory.talos.dev/metal-installer/376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba:latest # Allows for supplying the image used to perform the installation.
{{< /highlight >}}</details> | |
|`provisioning` |<a href="#UnattendedInstallConfig.provisioning">ProvisioningSpec</a> |The provisioning describes how the installation disk should be provisioned.  | |




## installer {#UnattendedInstallConfig.installer}

InstallerSpec describes the installer to perform the installation.




{{< highlight yaml >}}
installer:
    image: factory.talos.dev/metal-installer/376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba:latest # Allows for supplying the image used to perform the installation.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`image` |string |Allows for supplying the image used to perform the installation.<br>Image reference for each Talos release can be found on<br>[GitHub releases page](https://github.com/siderolabs/talos/releases).<br><br>If not set, it will run installer based on the current Talos version<br>and current schematic (this requires booting asset built by Image<br>Factory). <details><summary>Show example(s)</summary>{{< highlight yaml >}}
image: factory.talos.dev/metal-installer/376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba:latest
{{< /highlight >}}</details> | |






## provisioning {#UnattendedInstallConfig.provisioning}

ProvisioningSpec describes how the Physical Volumes are provisioned.





| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`diskSelector` |<a href="#UnattendedInstallConfig.provisioning.diskSelector">DiskSelectorSpec</a> |Matches disks to initialize as physical volumes.  | |
|`wipe` |bool |Indicates if the installation disk should be wiped at installation time.<br>Defaults to `true`.  |`true`<br />`yes`<br />`false`<br />`no`<br /> |




### diskSelector {#UnattendedInstallConfig.provisioning.diskSelector}

DiskSelectorSpec matches disks with CEL.





| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`match` |Expression |CEL expression matching a disk. <details><summary>Show example(s)</summary>match raw volume partitions labeled r-lvm*:{{< highlight yaml >}}
match: disk.dev_path == "/dev/sda"
{{< /highlight >}}</details> | |










