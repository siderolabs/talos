---
title: "The insecure flag"
description: "Learn how to use the insecure flag."
---

The `--insecure` flag is a per-command argument that allows you to communicate with the Talos API when a node is in maintenance mode, that is, before a node has been configured with a machine configuration or TLS certificates.

This is necessary because the machine configuration contains all the security credentials and PKI setup that secure communication between client and node.
Without these credentials, nodes in this maintenance state can only accept API calls through the insecure interface.

Only a small subset of Talos API commands support the `--insecure` flag, specifically those required for initial setup and maintenance operations.
However, once you've applied a machine config, you must stop using the `--insecure` flag for all subsequent operations.
The node will now expect secure communication through a talosconfig file.

**Note**: The `--insecure` flag is used in a different context by the `talosctl image cache-create` command.
This command is not used for interacting with the Talos node, but for allowing access to insecure image registries that do not support TLS.

## Supported Commands With the insecure Flag

The following commands can be used with the `--insecure` flag:

`talosctl apply-config`

Use this command alongside the `--insecure` flag to apply a machine configuration for the first time.

`talosctl version`

Check the Talos version running on the node.

`talosctl get`

Retrieves resources from the node.

**Note**: You **cannot** retrieve the following resources in insecure mode:

| **Resource Name**                                | **Aliases**                            |
| ------------------------------------------------ | -------------------------------------- |
| apicertificates.secrets.talos.dev                | apicertificate ac acs                  |
| auditpolicyconfigs.kubernetes.talos.dev          | auditpolicyconfig apc apcs             |
| certsans.secrets.talos.dev                       | certsan csan csans                     |
| deviceconfigspecs.net.talos.dev                  | deviceconfigspec dcs                   |
| discoveryconfigs.cluster.talos.dev               | discoveryconfig dc dcs                 |
| etcdrootsecrets.secrets.talos.dev                | etcdrootsecret ers                     |
| etcdsecrets.secrets.talos.dev                    | etcdsecret es                          |
| kubeletsecrets.secrets.talos.dev                 | kubeletsecret ks                       |
| kubernetesdynamiccerts.secrets.talos.dev         | kubernetesdynamiccert kdc kdcs         |
| kubernetesrootsecrets.secrets.talos.dev          | kubernetesrootsecret krs               |
| kubernetessecrets.secrets.talos.dev              | kubernetessecret ks                    |
| kubespanconfigs.kubespan.talos.dev               | kubespanconfig ksc kscs                |
| kubespanidentities.kubespan.talos.dev            | kubespanidentity ksi ksis              |
| linkspecs.net.talos.dev                          | linkspec ls                            |
| machineconfigs.config.talos.dev                  | machineconfig mc mcs                   |
| maintenancerootsecrets.secrets.talos.dev         | maintenancerootsecret mrs              |
| maintenanceservicecertificates.secrets.talos.dev | maintenanceservicecertificate msc mscs |
| nodeannotationspecs.k8s.talos.dev                | nodeannotationspec nas                 |
| nodecordonedspecs.k8s.talos.dev                  | nodecordonedspec ncs                   |
| nodelabelspecs.k8s.talos.dev                     | nodelabelspec nls                      |
| nodetaintspecs.k8s.talos.dev                     | nodetaintspec nts                      |
| operatorspecs.net.talos.dev                      | operatorspec os                        |
| osrootsecrets.secrets.talos.dev                  | osrootsecret osrs                      |
| siderolinkconfigs.siderolink.talos.dev           | siderolinkconfig sc scs                |
| siderolinktunnels.siderolink.talos.dev           | siderolinktunnel st sts                |
| trustdcertificates.secrets.talos.dev             | trustdcertificate tc tcs               |
| volumeconfigs.block.talos.dev                    | volumeconfig vc vcs                    |

`talosctl meta`

Manages key-value pairs in the META partition.

- `talosctl meta write`: Writes a key-value pair to the META partition.
- `talosctl meta delete`: Deletes keys from the META partition.

`talosctl reset`

Used alongside the `--insecure` flag to reset nodes in Omni.

`talosctl upgrade`

Used alongside the `--insecure` flag to upgrade Talos in Omni.

Refer to the [CLI reference]({{< relref "../../reference/cli" >}}) for full CLI details.

## Usage Example

Here is an example of how to use the `--insecure` flag:

```bash
# First time applying configuration (requires --insecure)
talosctl apply-config --insecure --nodes 192.168.1.100 --file controlplane.yaml
# After configuration is applied, subsequent commands are secure
talosctl get disks --nodes 192.168.1.100 --talosconfig=./talosconfig
```

## In Omni-Managed Clusters

The `--insecure` flag works differently when you're using Omni to manage Talos clusters.

Here, the flag is used for nodes that haven't joined a cluster yet.
These nodes will only listen for communication over the SideroLink connection, a secure VPN point-to-point connection between Omni and the Talos node.

So the SideroLink connection is the only way you can run commands against a node in insecure mode.

This architecture provides a unique security advantage because if a machine is managed by Omni, you cannot send configurations to it from another machine, even if they are on the same network.
This is because the Talos machine does not listen on any general network interface and only communicates with Omni through the secure SideroLink tunnel.
