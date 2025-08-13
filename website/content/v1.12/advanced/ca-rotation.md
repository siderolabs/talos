---
title: "CA Rotation"
description: "How to rotate Talos and Kubernetes API root certificate authorities."
---

In general, you almost never need to rotate the root CA certificate and key for the Talos API and Kubernetes API.
Talos sets up root certificate authorities with the lifetime of 10 years, and all Talos and Kubernetes API certificates are issued by these root CAs.
So the rotation of the root CA is only needed if:

- you suspect that the private key has been compromised;
- you want to revoke access to the cluster for a leaked `talosconfig` or `kubeconfig`;
- once in 10 years.

## Overview

There are some details which make Talos and Kubernetes API root CA rotation a bit different, but the general flow is the same:

- generate new CA certificate and key;
- add new CA certificate as 'accepted', so new certificates will be accepted as valid;
- swap issuing CA to the new one, old CA as accepted;
- refresh all certificates in the cluster;
- remove old CA from 'accepted'.

At the end of the flow, old CA is completely removed from the cluster, so all certificates issued by it will be considered invalid.

Both rotation flows are described in detail below.

## Talos API

### Automated Talos API CA Rotation

Talos API CA rotation doesn't interrupt connections within the cluster, and it doesn't require a reboot of the nodes.

Run the following command in dry-run mode to see the steps which will be taken:

```shell
$ talosctl -n <CONTROLPLANE> rotate-ca --dry-run=true --talos=true --kubernetes=false
> Starting Talos API PKI rotation, dry-run mode true...
> Using config context: "talos-default"
> Using Talos API endpoints: ["172.20.0.2"]
> Cluster topology:
  - control plane nodes: ["172.20.0.2"]
  - worker nodes: ["172.20.0.3"]
> Current Talos CA:
...
```

No changes will be done to the cluster in dry-run mode, so you can safely run it to see the steps.

Before proceeding, make sure that you can capture the output of `talosctl` command, as it will contain the new CA certificate and key.
Record a list of Talos API users to make sure they can all be updated with new `talosconfig`.

Run the following command to rotate the Talos API CA:

```shell
$ talosctl -n <CONTROLPLANE> rotate-ca --dry-run=false --talos=true --kubernetes=false
> Starting Talos API PKI rotation, dry-run mode false...
> Using config context: "talos-default-268"
> Using Talos API endpoints: ["172.20.0.2"]
> Cluster topology:
  - control plane nodes: ["172.20.0.2"]
  - worker nodes: ["172.20.0.3"]
> Current Talos CA:
...
> New Talos CA:
...
> Generating new talosconfig:
context: talos-default
contexts:
    talos-default:
        ....
> Verifying connectivity with existing PKI:
  - 172.20.0.2: OK (version {{< release >}})
  - 172.20.0.3: OK (version {{< release >}})
> Adding new Talos CA as accepted...
  - 172.20.0.2: OK
  - 172.20.0.3: OK
> Verifying connectivity with new client cert, but old server CA:
2024/04/17 21:26:07 retrying error: rpc error: code = Unavailable desc = connection error: desc = "error reading server preface: remote error: tls: unknown certificate authority"
  - 172.20.0.2: OK (version {{< release >}})
  - 172.20.0.3: OK (version {{< release >}})
> Making new Talos CA the issuing CA, old Talos CA the accepted CA...
  - 172.20.0.2: OK
  - 172.20.0.3: OK
> Verifying connectivity with new PKI:
2024/04/17 21:26:08 retrying error: rpc error: code = Unavailable desc = connection error: desc = "transport: authentication handshake failed: tls: failed to verify certificate: x509: certificate signed by unknown authority (possibly because of \"x509: Ed25519 verification failure\" while trying to verify candidate authority certificate \"talos\")"
  - 172.20.0.2: OK (version {{< release >}})
  - 172.20.0.3: OK (version {{< release >}})
> Removing old Talos CA from the accepted CAs...
  - 172.20.0.2: OK
  - 172.20.0.3: OK
> Verifying connectivity with new PKI:
  - 172.20.0.2: OK (version {{< release >}})
  - 172.20.0.3: OK (version {{< release >}})
> Writing new talosconfig to "talosconfig"
```

Once the rotation is done, stash the new Talos CA, update `secrets.yaml` (if using that for machine configuration generation) with new CA key and certificate.

The new client `talosconfig` is written to the current directory as `talosconfig`.
You can merge it to the default location with `talosctl config merge ./talosconfig`.

If other client access `talosconfig` files needs to be generated, use `talosctl config new` with new `talosconfig`.

> Note: if using [Talos API access from Kubernetes]({{< relref "./talos-api-access-from-k8s" >}}) feature, pods might need to be restarted manually to pick up new `talosconfig`.

### Manual Steps for Talos API CA Rotation

1. Generate new Talos CA (e.g. use `talosctl gen secrets` and use Talos CA).
2. Patch machine configuration on all nodes updating `.machine.acceptedCAs` with new CA certificate.
3. Generate `talosconfig` with client certificate generated with new CA, but still using old CA as server CA, verify connectivity, Talos should accept new client certificate.
4. Patch machine configuration on all nodes updating `.machine.ca` with new CA certificate and key, and keeping old CA certificate in `.machine.acceptedCAs` (on worker nodes `.machine.ca` doesn't have the key).
5. Generate `talosconfig` with both client certificate and server CA using new CA PKI, verify connectivity.
6. Remove old CA certificate from `.machine.acceptedCAs` on all nodes.
7. Verify connectivity.

## Kubernetes API

### Automated Kubernetes API CA Rotation

The automated process only rotates Kubernetes API CA, used by the `kube-apiserver`, `kubelet`, etc.
Other Kubernetes secrets might need to be rotated manually as required.
Kubernetes pods might need to be restarted to handle changes, and communication within the cluster might be disrupted during the rotation process.

Run the following command in dry-run mode to see the steps which will be taken:

```shell
$ talosctl -n <CONTROLPLANE> rotate-ca --dry-run=true --talos=false --kubernetes=true
> Starting Kubernetes API PKI rotation, dry-run mode true...
> Cluster topology:
  - control plane nodes: ["172.20.0.2"]
  - worker nodes: ["172.20.0.3"]
> Building current Kubernetes client...
> Current Kubernetes CA:
...
```

Before proceeding, make sure that you can capture the output of `talosctl` command, as it will contain the new CA certificate and key.
As Talos API access will not be disrupted, the changes can be reverted back if needed by reverting machine configuration.

Run the following command to rotate the Kubernetes API CA:

```shell
$ talosctl -n <CONTROLPLANE> rotate-ca --dry-run=false --talos=false --kubernetes=true
> Starting Kubernetes API PKI rotation, dry-run mode false...
> Cluster topology:
  - control plane nodes: ["172.20.0.2"]
  - worker nodes: ["172.20.0.3"]
> Building current Kubernetes client...
> Current Kubernetes CA:
...
> New Kubernetes CA:
...
> Verifying connectivity with existing PKI...
 - OK (2 nodes ready)
> Adding new Kubernetes CA as accepted...
  - 172.20.0.2: OK
  - 172.20.0.3: OK
> Making new Kubernetes CA the issuing CA, old Kubernetes CA the accepted CA...
  - 172.20.0.2: OK
  - 172.20.0.3: OK
> Building new Kubernetes client...
> Verifying connectivity with new PKI...
2024/04/17 21:45:52 retrying error: Get "https://172.20.0.1:6443/api/v1/nodes": EOF
 - OK (2 nodes ready)
> Removing old Kubernetes CA from the accepted CAs...
  - 172.20.0.2: OK
  - 172.20.0.3: OK
> Verifying connectivity with new PKI...
 - OK (2 nodes ready)
> Kubernetes CA rotation done, new 'kubeconfig' can be fetched with `talosctl kubeconfig`.
```

At the end of the process, Kubernetes control plane components will be restarted to pick up CA certificate changes.
Each node `kubelet` will re-join the cluster with new client certficiate.

New `kubeconfig` can be fetched with `talosctl kubeconfig` command from the cluster.

Kubernetes pods might need to be restarted manually to pick up changes to the Kubernetes API CA.

### Manual Steps for Kubernetes API CA Rotation

Steps are similar [to the Talos API CA rotation](#manual-steps-for-talos-api-ca-rotation), but use:

- `.cluster.acceptedCAs` in place of `.machine.acceptedCAs`;
- `.cluster.ca` in place of `.machine.ca`;
- `kubeconfig` in place of `talosconfig`.
