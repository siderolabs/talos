---
title: What's New in Talos 1.1
weight: 50
description: "List of new and shiny features in Talos Linux."
---

## Kubernetes

### Pod Security Admission

[Pod Security Admission](https://kubernetes.io/docs/concepts/security/pod-security-admission/) controller is enabled by default with the following policy:

```yaml
apiVersion: apiserver.config.k8s.io/v1
kind: AdmissionConfiguration
plugins:
- configuration:
    apiVersion: pod-security.admission.config.k8s.io/v1alpha1
    defaults:
      audit: restricted
      audit-version: latest
      enforce: baseline
      enforce-version: latest
      warn: restricted
      warn-version: latest
    exemptions:
      namespaces:
      - kube-system
      runtimeClasses: []
      usernames: []
    kind: PodSecurityConfiguration
  name: PodSecurity
  path: ""
```

The policy is part of the Talos machine configuration, and it can be modified to suite your needs.

### Kubernetes API Server Anonymous Auth

Anonymous authentication is now disabled by default for the `kube-apiserver` (CIS compliance).

To enable anonymous authentication, update the machine config with:

```yaml
cluster:
    apiServer:
        extraArgs:
            anonymous-auth: true
```

## Machine Configuration

### Apply Config `--dry-run`

The commands `talosctl apply-config`, `talosctl patch mc` and `talosctl edit mc` now support `--dry-run` flag.
If enabled it just prints out the selected config application mode and the configuration diff.

### Apply Config `--mode=try`

The commands `talosctl apply-config`, `talosctl patch mc` and `talosctl edit mc` now support the new mode called `try`.
In this mode the config change is applied for a period of time and then reverted back to the state it was before the change.
`--timeout` parameter can be used to customize the config rollback timeout.
This new mode can be used only with the parts of the config that can be changed without a reboot and can help to check that
the new configuration doesn't break the node.

Can be especially useful to check network interfaces changes that may lead to the loss of connectivity to the node.

## Networking

### Network Device Selector

Talos machine configuration supports specifying network interfaces by selectors instead of interface name.
See [documentation]({{< relref "../talos-guides/network/device-selector" >}}) for more details.

## SBCs

### RockPi 4 variants A and B

Talos now supports RockPi variants A and B in addition to RockPi 4C

### Raspberry Pi PoE Hat Fan

Talos now enables the Raspberry Pi PoE fan control by pulling in the poe overlay that works with upstream kernel

## Miscellaneous

### IPv6 in Docker-based Talos Clusters

The command `talosctl cluster create` now enables IPv6 by default for the Docker containers
created for Talos nodes.
This allows to use IPv6 addresses in Kubernetes networking.

If `talosctl cluster create` fails to work on Linux due to the lack of IPv6 support,
please use the flag `--disable-docker-ipv6` to revert the change.

### `eudev` Default Rules

Drops some default eudev rules that doesn't make sense in the context of Talos OS.
Especially the ones around sound devices, cd-roms and renaming the network interfaces to be predictable.
