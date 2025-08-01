---
title: "The insecure flag"
description: "Learn how to use the insecure flag."
---

The `--insecure` flag is a per-command argument that allows the `talosctl` client to communicate with the Talos API when a node is in maintenance mode, that is, before it has been configured with a machine configuration.

Talos normally uses mutual TLS (mTLS) for all API communications.
This means that both the `talosctl` client and the node verify each other’s identity using certificates provided in the machine configuration.

However, when a node is in maintenance mode, it still serves the Talos API over TLS, but with some key differences:

* The node uses a self-signed TLS certificate.
* The client (talosctl) does not present a certificate.
* Neither side can verify the other's identity.

In this case, the `--insecure` flag tells `talosctl` to skip verifying the server’s certificate, allowing the connection to proceed.

Only a small subset of Talos API commands support the --insecure flag, specifically those required for initial setup and maintenance operations.

However, once you've applied a machine config, you must stop using the `--insecure` flag for all subsequent operations.
The node will now expect secure communication through a talosconfig file.

**Note**: The `--insecure` flag is used in a different context by the `talosctl image cache-create` command.
This command is not used for interacting with the Talos node, but for allowing access to insecure image registries that do not support TLS.

## In Omni-Managed Clusters

The `--insecure` flag works differently when you're using Omni to manage Talos clusters.

Here, the flag is used for nodes that haven't joined a cluster yet.
These nodes will only listen for communication over the SideroLink connection, a secure VPN point-to-point connection between Omni and the Talos node.

So the SideroLink connection is the only way you can run commands against a node in insecure mode.

This architecture provides a unique security advantage because if a machine is managed by Omni, you cannot send configurations to it from another machine, even if they are on the same network.
This is because the Talos machine does not listen on any general network interface and only communicates with Omni through the secure SideroLink tunnel.

## Supported Commands With the insecure Flag

The following commands can be used with the --insecure flag:

`talosctl apply-config`

Use this command alongside the `--insecure` flag to apply a machine configuration for the first time.

`talosctl version`

Check the Talos version running on the node.

`talosctl get`

Retrieves resources from the node.
Verify which resources are retrievable in `--insecure` mode by following these steps:

1. Set your Talos node IP address as a variable (replace <node_ip> with the IP address of your Talos node):

    ```bash
    NODE_IP=<node_ip>

    ```

1. List resources available in `--insecure` mode:

    ```bash
    talosctl get rd --insecure --nodes $NODE_IP -o json \
    | jq -r 'select(.spec.sensitivity == null) | .spec.aliases[0]'

    ```

1. List resources not available in `--insecure` mode:

    ```bash
    talosctl get rd --insecure --nodes $NODE_IP -o json \
    | jq -r 'select(.spec.sensitivity != null) | .spec.aliases[0]'

    ```

`talosctl meta`

Manages key-value pairs in the META partition.

`talosctl reset`

Resets the nodes in Omni.

`talosctl upgrade`

Upgrades the Talos versions  in the nodes in Omni.

`talosctl wipe disk`

Erase data from disk partitions on a Talos node.

Refer to the [CLI reference](https://www.talos.dev/v1.10/reference/cli/) for full CLI details.

## Usage Example

Here is an example of how to use the `--insecure` flag in Talos:

```bash
# First time applying configuration (requires --insecure)

talosctl apply-config --insecure --nodes 192.168.1.100 --file controlplane.yaml

# After configuration is applied, subsequent commands are secure

talosctl get disks --nodes 192.168.1.100 --talosconfig=./talosconfig
```
