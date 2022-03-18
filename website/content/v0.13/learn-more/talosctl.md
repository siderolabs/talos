---
title: "talosctl"
weight: 7
---

The `talosctl` tool packs a lot of power into a small package.
It acts as a reference implementation for the Talos API, but it also handles a lot of
conveniences for the use of Talos and its clusters.

### Video Walkthrough

To see some live examples of talosctl usage, view the following video:

<iframe width="560" height="315" src="https://www.youtube.com/embed/pl0l_K_3Y6o" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>

## Client Configuration

Talosctl configuration is located in `$XDG_CONFIG_HOME/talos/config.yaml` if `$XDG_CONFIG_HOME` is defined.
Otherwise it is in `$HOME/.talos/config`.
The location can always be overridden by the `TALOSCONFIG` environment variable or the `--talosconfig` parameter.

Like `kubectl`, `talosctl` uses the concept of configuration contexts, so any number of Talos clusters can be managed with a single configuration file.
Unlike `kubectl`, it also comes with some intelligent tooling to manage the merging of new contexts into the config.
The default operation is a non-destructive merge, where if a context of the same name already exists in the file, the context to be added is renamed by appending an index number.
You can easily overwrite instead, as well.
See the `talosctl config help` for more information.

## Endpoints and Nodes

![Endpoints and Nodes](/images/endpoints-and-nodes.png)

The `endpoints` are the communication endpoints to which the client directly talks.
These can be load balancers, DNS hostnames, a list of IPs, etc.
Further, if multiple endpoints are specified, the client will automatically load
balance and fail over between them.
In general, it is recommended that these point to the set of control plane nodes, either directly or through a reverse proxy or load balancer.

Each endpoint will automatically proxy requests destined to another node through it, so it is not necessary to change the endpoint configuration just because you wish to talk to a different node within the cluster.

Endpoints _do_, however, need to be members of the same Talos cluster as the target node, because these proxied connections reply on certificate-based authentication.

The `node` is the target node on which you wish to perform the API call.
While you can configure the target node (or even set of target nodes) inside the 'talosctl' configuration file, it is often useful to simply and explicitly declare the target node(s) using the `-n` or `--nodes` command-line parameter.

Keep in mind, when specifying nodes that their IPs and/or hostnames are as seen by the endpoint servers, not as from the client.
This is because all connections are proxied first through the endpoints.

## Kubeconfig

The configuration for accessing a Talos Kubernetes cluster is obtained with `talosctl`.
By default, `talosctl` will safely merge the cluster into the default kubeconfig.
Like `talosctl` itself, in the event of a naming conflict, the new context name will be index-appended before insertion.
The `--force` option can be used to overwrite instead.

You can also specify an alternate path by supplying it as a positional parameter.

Thus, like Talos clusters themselves, `talosctl` makes it easy to manage any
number of kubernetes clusters from the same workstation.

## Commands

Please see the [CLI reference](../../reference/cli/) for the entire list of commands which are available from `talosctl`.
