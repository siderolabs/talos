---
title: "SBOMs"
description: "A guide on using Software Bill of Materials for Talos Linux."
---

Software Bill of Materials (SBOM) is a formal record containing the details and supply chain relationships of various components used in building a software product.
SBOMs are used to provide transparency and traceability of software components, which is essential for security, compliance, and efficient management of software supply chains.

Talos Linux provides SBOMs for core operating system components, including the Linux kernel, built-in components like `containerd`, and other software packages used to build Talos Linux.
When a system extension is installed, it can also provide its own SBOM, which will be included in the overall SBOM for the Talos Linux system.

## Acquiring SBOMs

SBOMs for Talos Linux are provided in SPDX format, which is a standard format for representing SBOMs.
You can acquire SBOMs for Talos Linux in the following ways:

* Download the SBOM for a specific Talos Linux release from the [GitHub release](https://github.com/siderolabs/talos/releases/tag/{{< release >}}) page:
  * `talos-amd64.spdx.json` for the amd64 architecture.
  * `talos-arm64.spdx.json` for the arm64 architecture.
* Acquire the SBOM from a running Talos Linux system using the `talosctl` command:
  * core Talos Linux SBOM in the `/usr/share/spdx` directory.
  * extension SBOMs in the `/usr/local/share/spdx` directory.

## SBOMs as Resources

Talos Linux SBOMs are also available as resources in the Talos Linux system.
You can access the SBOMs using the `talosctl` command:

```bash
talosctl get sboms
NODE         NAMESPACE   TYPE       ID              VERSION   VERSION                LICENSE
172.20.0.2   runtime     SBOMItem   Talos           1         {{< release >}}
172.20.0.2   runtime     SBOMItem   apparmor        1         v3.1.7                 GPL-2.0-or-later
172.20.0.2   runtime     SBOMItem   cel.dev/expr    1         v0.24.0
...
```

You can also get the SBOM for a specific component using the `talosctl get sbom` command:

```yaml
# talosctl get sbom kernel -o yaml
node: 172.20.0.2
metadata:
    namespace: runtime
    type: SBOMItems.talos.dev
    id: kernel
    version: 1
    owner: runtime.SBOMItemController
    phase: running
    created: 2025-07-24T14:20:29Z
    updated: 2025-07-24T14:20:29Z
spec:
    name: kernel
    version: 6.12.38
    license: GPL-2.0-only
    cpes:
        - cpe:2.3:o:linux:linux_kernel:6.12.38:*:*:*:*:*:*:*
```

## Scanning SBOMs

You can scan SBOMs for known vulnerabilities using tools like [Grype](https://github.com/anchore/grype).
You will need two source files for scanning:

* The SBOM file in SPDX format.
* The vulnerability exclusion database (VEX).

VEX database is used to filter out vulnerabilities that are not applicable to the specific software version or configuration,
which helps to reduce false positives in vulnerability scanning.

In order to generate the VEX database, run the following command:

```bash
docker run --rm --pull always ghcr.io/siderolabs/generate-vex:latest gen --target-version {{< release >}} > vex.json
```

The basic command to scan the SBOM is as follows:

```bash
grype sbom:talos-amd64.spdx.json
```

With VEX database, the command becomes:

> Note: At the moment of writing, the scan with VEX database fails until this [PR](https://github.com/anchore/grype/pull/2798) is merged.

```bash
grype sbom:talos-amd64.spdx.json --vex vex.json
```
