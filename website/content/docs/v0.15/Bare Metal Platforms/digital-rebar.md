---
title: "Digital Rebar"
description: "In this guide we will create an Kubernetes cluster with 1 worker node, and 2 controlplane nodes using an existing digital rebar deployment."
---

## Prerequisites

- 3 nodes (please see [hardware requirements](../../guides/getting-started#system-requirements))
- Loadbalancer
- Digital Rebar Server
- Talosctl access (see [talosctl setup](../../guides/getting-started/talosctl))

## Creating a Cluster

In this guide we will create an Kubernetes cluster with 1 worker node, and 2 controlplane nodes.
We assume an existing digital rebar deployment, and some familiarity with iPXE.

We leave it up to the user to decide if they would like to use static networking, or DHCP.
The setup and configuration of DHCP will not be covered.

### Create the Machine Configuration Files

#### Generating Base Configurations

Using the DNS name of the load balancer, generate the base configuration files for the Talos machines:

```bash
$ talosctl gen config talos-k8s-metal-tutorial https://<load balancer IP or DNS>:<port>
created controlplane.yaml
created worker.yaml
created talosconfig
```

> The loadbalancer is used to distribute the load across multiple controlplane nodes.
> This isn't covered in detail, because we assume some loadbalancing knowledge before hand.
> If you think this should be added to the docs, please [create a issue](https://github.com/talos-systems/talos/issues).

At this point, you can modify the generated configs to your liking.
Optionally, you can specify `--config-patch` with RFC6902 jsonpatch which will be applied during the config generation.

#### Validate the Configuration Files

```bash
$ talosctl validate --config controlplane.yaml --mode metal
controlplane.yaml is valid for metal mode
$ talosctl validate --config worker.yaml --mode metal
worker.yaml is valid for metal mode
```

#### Publishing the Machine Configuration Files

Digital Rebar has a build-in fileserver, which means we can use this feature to expose the talos configuration files.
We will place `controlplane.yaml`, and `worker.yaml` into Digital Rebar file server by using the `drpcli` tools.

Copy the generated files from the step above into your Digital Rebar installation.

```bash
drpcli file upload <file>.yaml as <file>.yaml
```

Replacing `<file>` with controlplane or worker.

### Download the boot files

Download a recent version of `boot.tar.gz` from [github.](https://github.com/talos-systems/talos/releases/)

Upload to DRB:

```bash
$ drpcli isos upload boot.tar.gz as talos.tar.gz
{
  "Path": "talos.tar.gz",
  "Size": 96470072
}
```

We have some Digital Rebar [example files](https://github.com/talos-systems/talos/tree/master/hack/test/digitalrebar/) in the Git repo you can use to provision Digital Rebar with drpcli.

To apply these configs you need to create them, and then apply them as follow:

```bash
$ drpcli bootenvs create talos
{
  "Available": true,
  "BootParams": "",
  "Bundle": "",
  "Description": "",
  "Documentation": "",
  "Endpoint": "",
  "Errors": [],
  "Initrds": [],
  "Kernel": "",
  "Meta": {},
  "Name": "talos",
  "OS": {
    "Codename": "",
    "Family": "",
    "IsoFile": "",
    "IsoSha256": "",
    "IsoUrl": "",
    "Name": "",
    "SupportedArchitectures": {},
    "Version": ""
  },
  "OnlyUnknown": false,
  "OptionalParams": [],
  "ReadOnly": false,
  "RequiredParams": [],
  "Templates": [],
  "Validated": true
}
```

```bash
drpcli bootenvs update talos - < bootenv.yaml
```

> You need to do this for all files in the example directory.
> If you don't have access to the `drpcli` tools you can also use the webinterface.

It's important to have a corresponding SHA256 hash matching the boot.tar.gz

#### Bootenv BootParams

We're using some of Digital Rebar build in templating to make sure the machine gets the correct role assigned.

`talos.platform=metal talos.config={{ .ProvisionerURL }}/files/{{.Param \"talos/role\"}}.yaml"`

This is why we also include a `params.yaml` in the example directory to make sure the role is set to one of the following:

- controlplane
- worker

The `{{.Param \"talos/role\"}}` then gets populated with one of the above roles.

### Boot the Machines

In the UI of Digital Rebar you need to select the machines you want te provision.
Once selected, you need to assign to following:

- Profile
- Workflow

This will provision the Stage and Bootenv with the talos values.
Once this is done, you can boot the machine.

To understand the boot process, we have a higher level overview located at [metal overview](../overview).

### Bootstrap Etcd

To configure `talosctl` we will need the first control plane node's IP:

Set the `endpoints` and `nodes`:

```bash
talosctl --talosconfig talosconfig config endpoint <control plane 1 IP>
talosctl --talosconfig talosconfig config node <control plane 1 IP>
```

Bootstrap `etcd`:

```bash
talosctl --talosconfig talosconfig bootstrap
```

### Retrieve the `kubeconfig`

At this point we can retrieve the admin `kubeconfig` by running:

```bash
talosctl --talosconfig talosconfig kubeconfig .
```
