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
- `init_on_alloc=1`: required by KSPP
- `init_on_free=1`: required by KSPP
- `slab_nomerge`: required by KSPP
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

You can also override which configuration `talosctl` uses by specifying the `--talosconfig` parameter:

```bash
talosctl --talosconfig talosconfig
```

Configuring the endpoints:

```bash
talosctl config endpoint <endpoint>...
```

Endpoints are the communication endpoints to which the client directly talks.
These can be load balancers, DNS hostnames, a list of IPs, etc.
In general, it is recommended that these point to the set of control plane
nodes, either directly or through a reverse proxy or load balancer.

Each endpoint will automatically proxy requests destined to another node through
it, so it is not necessary to change the endpoint configuration just because you
wish to talk to a different node within the cluster.

Endpoints _do_, however, need to be members of the same Talos cluster as the
target node, because these proxied connections reply on certificate-based
authentication.

Configuring the nodes:

```bash
talosctl config nodes <node>...
```

The node is the target node on which you wish to perform the API call.
While you can configure the target node (or even set of target nodes) inside the
'talosctl' configuration file, it is often useful to simply and explicitly
declare the target node(s) using the `-n` or `--nodes` command-line parameter.

Keep in mind, when specifying nodes that their IPs and/or hostnames are as seen by the endpoint servers, not as from the client.
This is because all connections are proxied first through the endpoints.

To verify what node(s) you're currently talking to, you can run:

```bash
$ talosctl version
Client:
        ...
Server:
        NODE:        <node>
        ...
```
