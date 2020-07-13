---
title: 'apid'
---

When interacting with Talos, the gRPC api endpoint you will interact with directly is `apid`.
Apid acts as the gateway for all component interactions.
Apid provides a mechanism to route requests to the appropriate destination when running on a control plane node.

We'll use some examples below to illustrate what `apid` is doing.

When a user wants to interact with a Talos component via `talosctl`, there are two flags that control the interaction with `apid`.
The `-e | --endpoints` flag is used to denote which Talos node ( via `apid` ) should handle the connection.
Typically this is a public facing server.
The `-n | --nodes` flag is used to denote which Talos node(s) should respond to the request.
If `--nodes` is not specified, the first endpoint will be used.

> Note: Typically there will be an `endpoint` already defined in the Talos config file.
> Optionally, `nodes` can be included here as well.

For example, if a user wants to interact with `machined`, a command like `talosctl -e cluster.talos.dev memory` may be used.

```bash
$ talosctl -e cluster.talos.dev memory
NODE                TOTAL   USED   FREE   SHARED   BUFFERS   CACHE   AVAILABLE
cluster.talos.dev   7938    1768   2390   145      53        3724    6571
```

In this case, `talosctl` is interacting with `apid` running on `cluster.talos.dev` and forwarding the request to the `machined` api.

If we wanted to extend our example to retrieve `memory` from another node in our cluster, we could use the command `talosctl -e cluster.talos.dev -n node02 memory`.

```bash
$ talosctl -e cluster.talos.dev -n node02 memory
NODE    TOTAL   USED   FREE   SHARED   BUFFERS   CACHE   AVAILABLE
node02  7938    1768   2390   145      53        3724    6571
```

The `apid` instance on `cluster.talos.dev` receives the request and forwards it to `apid` running on `node02` which forwards the request to the `machined` api.

We can further extend our example to retrieve `memory` for all nodes in our cluster by appending additional `-n node` flags or using a comma separated list of nodes ( `-n node01,node02,node03` ):

```bash
$ talosctl -e cluster.talos.dev -n node01 -n node02 -n node03 memory
NODE     TOTAL    USED    FREE     SHARED   BUFFERS   CACHE   AVAILABLE
node01   7938     871     4071     137      49        2945    7042
node02   257844   14408   190796   18138    49        52589   227492
node03   257844   1830    255186   125      49        777     254556
```

The `apid` instance on `cluster.talos.dev` receives the request and forwards is to `node01`, `node02`, and `node03` which then forwards the request to their local `machined` api.
