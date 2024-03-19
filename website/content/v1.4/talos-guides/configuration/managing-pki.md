---
title: "Managing Talos PKI"
description: "How to manage Public Key Infrastructure"
aliases:
  - ../../guides/managing-pki
---

## Generating New Client Configuration

### Using Controlplane Node

If you have a valid (not expired) `talosconfig` with `os:admin` role,
a new client configuration file can be generated with `talosctl config new` against
any controlplane node:

```shell
talosctl -n CP1 config new talosconfig-reader --roles os:reader --crt-ttl 24h
```

A specific [role]({{< relref "rbac" >}}) and certificate lifetime can be specified.

### From Secrets Bundle

If a secrets bundle (`secrets.yaml` from `talosctl gen secrets`) was saved while
[generating machine configuration]({{< relref "../../introduction/getting-started/#configure-talos ">}}):

```shell
talosctl gen config --with-secrets secrets.yaml --output-types talosconfig -o talosconfig <cluster-name> https://<cluster-endpoint>
```

> Note: `<cluster-name>` and `<cluster-endpoint>` arguments don't matter, as they are not used for `talosconfig`.

### From Control Plane Machine Configuration

In order to create a new key pair for client configuration, you will need the root Talos API CA.
The base64 encoded CA can be found in the control plane node's configuration file.
Save the CA public key, and CA private key as `ca.crt`, and `ca.key` respectively:

```shell
yq eval .machine.ca.crt controlplane.yaml | base64 -d > ca.crt
yq eval .machine.ca.key controlplane.yaml | base64 -d > ca.key
```

Now, run the following commands to generate a certificate:

```bash
talosctl gen key --name admin
talosctl gen csr --key admin.key --ip 127.0.0.1
talosctl gen crt --ca ca --csr admin.csr --name admin
```

Put the base64-encoded files to the respective location to the `talosconfig`:

```yaml
context: mycluster
contexts:
    mycluster:
        endpoints:
            - CP1
            - CP2
        ca: <base64-encoded ca.crt>
        crt: <base64-encoded admin.crt>
        key: <base64-encoded admin.key>
```

## Renewing an Expired Administrator Certificate

By default admin `talosconfig` certificate is valid for 365 days, while cluster CAs are valid for 10 years.
In order to prevent admin `talosconfig` from expiring, renew the client config before expiration using `talosctl config new` command described above.

If the `talosconfig` is expired or lost, you can still generate a new one using either the `secrets.yaml`
secrets bundle or the control plane node's configuration file using methods described above.
