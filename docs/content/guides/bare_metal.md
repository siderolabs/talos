---
title: Bare Metal
date: 2019-06-21T06:25:46-08:00
draft: false
menu:
  docs:
    parent: 'guides'
---

## Generate configuration

When considering Talos for production, the best way to get started is by using the `osctl config generate` command.

Talos requires 3 static IPs, one for each of the master nodes. After allocating these addresses, you can generate the necessary configs with the following commands:

```bash
osctl config generate <cluster name> <master-1 ip,master-2 ip, master-3 ip>
```

This will generate 5 files - `master-{1,2,3}.yaml`, `worker.yaml`, and `talosconfig`. The master and worker config files contain just enough config to bootstrap your cluster, and can be further customized as necessary.

These config files should be supplied as machine userdata or some internally accessible url so they can be downloaded during machine bootup. When specifying a remote location to download userdata from, the kernel parameter `talos.autonomy.io/userdata=http://myurl.com`.

An iPXE server such as [coreos/Matchbox](https://github.com/poseidon/matchbox) is recommended.

## Cluster interaction

After the machines have booted up, you'll want to manage your Talos config file.

The `osctl` tool looks for its configuration in `~/.talos/config` by default. The configuration file location can also be specified at runtime via `osctl --talosconfig myconfigfile`. In the previous step, the Talos configuration was generated in your working directory as `talosconfig`.

By default, the Talos configuration points to a single node. This can be overridden at runtime via `--target <ip>` flag so you can point to another node in your cluster.

Next, we'll need to generate the kubeconfig for our cluster. This can be achieved by runng `osctl kubeconfig`.

## Finalizing Kubernetes Setup

Once your machines boot up, you will want to apply a Pod Security Policy (PSP). There is a basic example that can be found [here](https://raw.githubusercontent.com/talos-systems/talos/master/hack/dev/manifests/psp.yaml) or you can create your own.

Finally, you'll want to apply a CNI plugin. You'll want to take note of the kubeadm `networking.podsubnet` parameter and ensure the network range matches up.