---
title: Getting Started
weight: 3
---

Regardless of where you run Talos, you will find that there is a pattern to deploying it.

In general you will need to:

- identity and create the image
- optionally create a load balancer for Kubernetes
- configure Talos
- create the nodes

## Kernel Parameters

The following is a list of kernel parameters required by Talos:

- `talos.config`: the HTTP(S) URL at which the machine data can be found
- `talos.platform`: can be one of `aws`, `azure`, `container`, `digitalocean`, `gcp`, `metal`, `packet`, or `vmware`
- `page_poison=1`: required by KSPP
- `slab_nomerge`: required by KSPP
- `slub_debug=P`: required by KSPP
- `pti=on`: required by KSPP

## CLI

### Installation

```bash
curl -Lo /usr/local/bin/talosctl https://github.com/talos-systems/talos/releases/latest/download/talosctl-$(uname -s | tr "[:upper:]" "[:lower:]")-amd64
chmod +x /usr/local/bin/talosctl
```

### Configuration

The `talosctl` command needs some configuration options to connect to the right node.
By default `talosctl` looks for a file called `config` located at `$HOME/.talos`.

You can also override which configuration `talosctl` uses by specifing the `--talosconfig` parameter:

```bash
talosctl --talosconfig talosconfig
```

Configuring the endpoints:

```bash
talosctl config endpoint <endpoint>...
```

Configuring the nodes:

```bash
talosctl config nodes <node>...
```

To verify what node you're currently connected to, you can run:

```bash
$ talosctl version
Client:
        ...
Server:
        NODE:        <node>
        ...
```
