---
title: "Vultr"
description: "Creating a cluster via the CLI (vultr-cli) on Vultr.com."
aliases:
  - ../../../cloud-platforms/vultr
---

## Creating a Cluster using the Vultr CLI

This guide will demonstrate how to create a highly-available Kubernetes cluster with one worker using the Vultr cloud provider.

[Vultr](https://www.vultr.com/) have a very well documented REST API, and an open-source [CLI](https://github.com/vultr/vultr-cli) tool to interact with the API which will be used in this guide.
Make sure to follow installation and authentication instructions for the `vultr-cli` tool.

### Boot Options

#### Upload an ISO Image

First step is to make the Talos ISO available to Vultr by uploading the latest release of the ISO to the Vultr ISO server.

```bash
vultr-cli iso create --url https://factory.talos.dev/image/376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba/{{< release >}}vultr-amd64.iso
```

Make a note of the `ID` in the output, it will be needed later when creating the instances.met

#### PXE Booting via Image Factory

Talos Linux can be PXE-booted on Vultr using [Image Factory]({{< relref "../../../learn-more/image-factory" >}}), using the `vultr` platform: e.g.
`https://pxe.factory.talos.dev/pxe/376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba/{{< release >}}/vultr-amd64` (this URL references the default schematic and `amd64` architecture).

Make a note of the `ID` in the output, it will be needed later when creating the instances.

### Create a Load Balancer

A load balancer is needed to serve as the Kubernetes endpoint for the cluster.

```bash
vultr-cli load-balancer create \
   --region $REGION \
   --label "Talos Kubernetes Endpoint" \
   --port 6443 \
   --protocol tcp \
   --check-interval 10 \
   --response-timeout 5 \
   --healthy-threshold 5 \
   --unhealthy-threshold 3 \
   --forwarding-rules frontend_protocol:tcp,frontend_port:443,backend_protocol:tcp,backend_port:6443
```

Make a note of the `ID` of the load balancer from the output of the above command, it will be needed after the control plane instances are created.

```bash
vultr-cli load-balancer get $LOAD_BALANCER_ID | grep ^IP
```

Make a note of the `IP` address, it will be needed later when generating the configuration.

### Create the Machine Configuration

#### Generate Base Configuration

Using the IP address (or DNS name if one was created) of the load balancer created above, generate the machine configuration files for the new cluster.

```bash
talosctl gen config talos-kubernetes-vultr https://$LOAD_BALANCER_ADDRESS
```

Once generated, the machine configuration can be modified as necessary for the new cluster, for instance updating disk installation, or adding SANs for the certificates.

#### Validate the Configuration Files

```bash
talosctl validate --config controlplane.yaml --mode cloud
talosctl validate --config worker.yaml --mode cloud
```

### Create the Nodes

#### Create the Control Plane Nodes

First a control plane needs to be created, with the example below creating 3 instances in a loop.
The instance type (noted by the `--plan vc2-2c-4gb` argument) in the example is for a minimum-spec control plane node, and should be updated to suit the cluster being created.

```bash
for id in $(seq 3); do
    vultr-cli instance create \
        --plan vc2-2c-4gb \
        --region $REGION \
        --iso $TALOS_ISO_ID \
        --host talos-k8s-cp${id} \
        --label "Talos Kubernetes Control Plane" \
        --tags talos,kubernetes,control-plane
done
```

Make a note of the instance `ID`s, as they are needed to attach to the load balancer created earlier.

```bash
vultr-cli load-balancer update $LOAD_BALANCER_ID --instances $CONTROL_PLANE_1_ID,$CONTROL_PLANE_2_ID,$CONTROL_PLANE_3_ID
```

Once the nodes are booted and waiting in maintenance mode, the machine configuration can be applied to each one in turn.

```bash
talosctl --talosconfig talosconfig apply-config --insecure --nodes $CONTROL_PLANE_1_ADDRESS --file controlplane.yaml
talosctl --talosconfig talosconfig apply-config --insecure --nodes $CONTROL_PLANE_2_ADDRESS --file controlplane.yaml
talosctl --talosconfig talosconfig apply-config --insecure --nodes $CONTROL_PLANE_3_ADDRESS --file controlplane.yaml
```

#### Create the Worker Nodes

Now worker nodes can be created and configured in a similar way to the control plane nodes, the difference being mainly in the machine configuration file.
Note that like with the control plane nodes, the instance type (here set by `--plan vc2-1-1gb`) should be changed for the actual cluster requirements.

```bash
for id in $(seq 1); do
    vultr-cli instance create \
        --plan vc2-1c-1gb \
        --region $REGION \
        --iso $TALOS_ISO_ID \
        --host talos-k8s-worker${id} \
        --label "Talos Kubernetes Worker" \
        --tags talos,kubernetes,worker
done
```

Once the worker is booted and in maintenance mode, the machine configuration can be applied in the following manner.

```bash
talosctl --talosconfig talosconfig apply-config --insecure --nodes $WORKER_1_ADDRESS --file worker.yaml
```

### Bootstrap etcd

Once all the cluster nodes are correctly configured, the cluster can be bootstrapped to become functional.
It is important that the `talosctl bootstrap` command be executed only once and against only a single control plane node.

```bash
talosctl --talosconfig talosconfig bootstrap --endpoints $CONTROL_PLANE_1_ADDRESS --nodes $CONTROL_PLANE_1_ADDRESS
```

### Configure Endpoints and Nodes

While the cluster goes through the bootstrapping process and beings to self-manage, the `talosconfig` can be updated with the [endpoints and nodes]({{< relref "../../../learn-more/talosctl#endpoints-and-nodes" >}}).

```bash
talosctl --talosconfig talosconfig config endpoints $CONTROL_PLANE_1_ADDRESS $CONTROL_PLANE_2_ADDRESS $CONTROL_PLANE_3_ADDRESS
talosctl --talosconfig talosconfig config nodes $CONTROL_PLANE_1_ADDRESS $CONTROL_PLANE_2_ADDRESS $CONTROL_PLANE_3_ADDRESS WORKER_1_ADDRESS
```

### Retrieve the `kubeconfig`

Finally, with the cluster fully running, the administrative `kubeconfig` can be retrieved from the Talos API to be saved locally.

```bash
talosctl --talosconfig talosconfig kubeconfig .
```

Now the `kubeconfig` can be used by any of the usual Kubernetes tools to interact with the Talos-based Kubernetes cluster as normal.
