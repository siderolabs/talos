---
title: Production Clusters
weight: 30
description: "Recommendations for setting up a Talos Linux cluster in production."
---

This guide explains things to consider to create a production quality Talos Linux cluster for bare metal. Check out the [Reference Architecture documentation](https://www.siderolabs.com/resource-hub/resources/) for architectural diagrams and guidance on creating production-grade clusters in other environments.
 
This guide assumes that you’ve already created a development cluster and are familiar with the **Getting Started** documentation. If not, please refer to the [Getting Started]({{< relref "getting-started" >}}) guide for more information.

When moving from a learning environment to a production-ready Talos Linux cluster, you have to consider several critical factors:

- High availability for your control plane nodes.
- Secure configuration management.
- Reliability for continuous service and minimal downtime.
- Authentication for access control.

Follow the steps below to build a production-grade Talos cluster that is highly available, reliable, and secure.

**Note**: Check out Omni for managing large-scale Talos Linux clusters automatically.


## Step 1: Prepare Your Infrastructure

To create your production cluster infrastructure :

1. Boot your machines using the Talos ISO image
2. Ensure network access on your nodes.

Here is how to do each step:

### Boot Your Machines Using the Talos ISO Image

Follow these steps to boot your machines using the Talos ISO image:

1. Download the latest ISO for your infrastructure depending on the hardware type from the [Talos Image factory](https://factory.talos.dev/). 

**Note**: For network booting and self-built media using published kernel there are a number of required kernel parameters. Please see the [kernel docs]({{< relref "../reference/kernel" >}}) getting-started for more information.


2. Boot three control planes using the ISO image you just downloaded. 
3. Boot additional machines as worker nodes.

### Ensure Network Access 

If your nodes are behind a firewall, in a private network, or otherwise not directly reachable, you would need to configure a load balancer to forward TCP port 50000 to reach the nodes for Talos API access.

**Note**: Because the Talos Linux API uses gRPC and mutual TLS, it cannot be proxied by a HTTP/S proxy, but only by a TCP load balancer.

With your control plane and worker nodes booted, next configure your Kubernetes endpoint.

## Step 2: Store Your IP Addresses in a Variable

To store variables for your machines’ IP addresses:

1. Copy the IP address displayed on each machine console, including the control plane and any worker nodes you’ve created. 
    
    If you don’t have a display connected, retrieve the IP addresses from your DHCP server.

![IP address display](/images/IP-address-install-display.png)



2. Create a Bash array for your control plane node IP addresses, replacing each `<control-plane-ip>` placeholder with the IP address of a control plane node. You can include as many IP addresses as needed:

```bash
CONTROL_PLANE_IP=("<control-plane-ip-1>" "<control-plane-ip-1>" "<control-plane-ip-2>")
```
**For example**:

If your control plane nodes IP addresses are `192.168.0.2`, `192.168.0.3`, `192.168.0.4`, your command would be:

```bash
CONTROL_PLANE_IP= ("192.168.0.2" "192.168.0.3" "192.168.0.4")
```


3. If you have worker nodes, store their IP addresses in a Bash array. Replace each `<worker-ip>` placeholder with the actual IP address of a worker node. You can include as many IP addresses as needed:

```bash
WORKER_IP=("<worker-ip-1>" "<worker-ip-2>" "<worker-ip-3>")
```


## Step 3: Decide Your Kubernetes Endpoint

You've set up multiple control planes for high availability, but they only provide true high availability if the Kubernetes API server endpoint can reach all control plane nodes.

Here are two common ways to configure this:

- **Dedicated load balancer**: Set a dedicated load balancer that route to your control plane nodes.
- **DNS records**: Create multiple DNS records that point to all your control plane nodes

With these, you can pass in one IP address or DNS name during setup that route to all your control plane nodes. 

Here is how you can configure each option:

### Dedicated Load Balancer

If you're using a cloud provider or have your own load balancer (such as HAProxy, an NGINX reverse proxy, or an F5 load balancer), setting up a dedicated load balancer is a natural choice. 

It is also important to note that if you [created the cluster with Omni](https://omni.siderolabs.com/tutorials/getting_started), Omni will automatically be a load balancer for your Kubernetes endpoint.

Configure a frontend to listen on TCP port 6443 and direct traffic to the addresses of your Talos control plane nodes. 

Your Kubernetes endpoint will be the IP address or DNS name of the load balancer's frontend, with the port appended, for example, `https://myK8s.mydomain.io:6443`.

**Note**: You cannot use a HTTP load balancer, because the Kubernetes API server handles TLS termination and mutual TLS authentication.

### DNS Records

Additionally, you can configure your Kubernetes endpoint using DNS records. Simply, add multiple A or AAAA records, one for each control plane, to a DNS name.

For example, you can add:

```
kube.cluster1.mydomain.com  IN  A  192.168.0.10
kube.cluster1.mydomain.com  IN  A  192.168.0.11
kube.cluster1.mydomain.com  IN  A  192.168.0.12
```

Then your endpoint would be:

```
https://kube.cluster1.mydomain.com:6443
```

## Step 4: Save Your Endpoint in a Variable

Set a variable to store the endpoint you chose in Step 3. Replace `<your_endpoint>` placeholder with your actual endpoint:

```bash
export YOUR_ENDPOINT=<your_endpoint>
```

## Step 5: Generate Secrets Bundle

The secrets bundle is a file that contains all the cryptographic keys, certificates, and tokens needed to secure your Talos Linux cluster.

To generate the secrets bundle, run:
```bash
talosctl gen secrets -o secrets.yaml
```

## Step 6: Generate Machine Configurations

Follow these steps to generate machine configuration:

1.  Set a variable for your cluster name by running the following command. Replace `<your_cluster_name>` with the name you want to give your cluster:

```bash
export CLUSTER_NAME=<your_cluster_name>
```

2. Run this command to generate your machine configuration files using your secrets bundle:

```bash
talosctl gen config --with-secrets secrets.yaml $CLUSTER_NAME https://$YOUR_ENDPOINT:6443
```
This command will generate three files:

- **controlplane.yaml**: Configuration for your control plane.
- **worker.yaml**: Configuration for your worker nodes.
- **talosconfig**: The `talosctl` configuration file used to connect to and authenticate with your cluster.

##  Step 7: Unmount the ISO

Unplug your installation USB drive or unmount the ISO from all your control plane and worker nodes. This prevents you from accidentally installing to the USB drive and makes it clearer which disk to select for installation.

## Step 8: Understand Your Nodes

The default machine configurations for control plane and worker nodes are typically sufficient to get your cluster running. However, you may need to customize certain settings such as network interfaces and disk configurations depending on your specific environment.

Follow these steps to verify that your machine configurations are set up correctly:

1. **Check network interfaces**: Run this command to view all network interfaces on any node, whether control plane or worker. 


    Replace `<node-ip-address>` with the IP of the node you want to inspect.


    **Note**: Copy the network ID with an Operational state (OPER) value of **up**.

```bash
talosctl --nodes <node-ip-address> get links --insecure
```


2. **Check Available Disks:** Run this command to check all available disks on any node. Replace `<node-ip-address>` with the IP address of the node you want to inspect:

```bash  
talosctl get disks --insecure --nodes <node-ip-address>
```

3. **Verify Configuration Files:** Open your `worker.yaml` and `controlplane.yaml` configuration files in your preferred editor. Check that the values match your worker and control plane node's network and disk settings. If the values don't match, you'll need to update your machine configuration..


    **Note**: Refer to the [Talos CLI reference]({{< relref "../reference/cli" >}}) for additional commands to gather more information about your nodes and cluster.


## Step 9: Patch Your Machine Configuration (Optional)

You can patch your worker and control plane machine configuration to reflect the correct network interface and disk of your control plane nodes.

Follow these steps to patch your machine configuration:

1. Create patch files for the configurations you want to modify:

```bash
touch controlplane-patch-1.yaml # For patching the control plane nodes configuration 
touch worker-patch-1.yaml # For patching the worker nodes configuration 
```
**Note**: You don't have to create both patch files, only create patches for the configurations you actually need to modify. 

You can also create multiple patch files (e.g., `controlplane-patch-2.yaml`, `controlplane-patch-3.yaml`) if you want to make multiple subsequent patches to the same machine configuration.


2. Copy and paste this YAML block of code and add the correct hardware values to each patch file.  


    For example, for `controlplane-patch-1` use the network interface and disk information you gathered from your control plane nodes :

```yaml
machine:
  network: 
    interfaces:
      - interface: <control-plane-network-interface>  # From control plane node
        dhcp: true
  install:
    disk: /dev/<control-plane-disk-name> # From control plane node
```

For `worker-patch-1.yaml`, use network interface and disk information from your worker nodes:

```yaml
machine:
  network: 
    interfaces:
      - interface: <worker-network-interface>  # From worker node
        dhcp: true
  install:
    disk: /dev/<worker-disk-name> # From worker node
```

3. Apply the different patch files for the different machine configuration:
    - **For control plane**:

```bash
 talosctl machineconfig patch controlplane.yaml --patch @controlplane-patch-1.yaml --output controlplane.yaml
```

  - **For worker**: 

```bash
talosctl machineconfig patch worker.yaml --patch @worker-patch-1.yaml --output worker.yaml
```
Additionally, you can learn more about [patches]({{< relref "../talos-guides/configuration/patching/" >}}) from the configuration patches documentation.

## Step 10: Apply the Machine Configuration

To apply your machine configuration:

1. Run this command to apply the `controlplane.yaml` configuration to your control plane nodes:

```bash
for ip in "${CONTROL_PLANE_IP[@]}"; do
  echo "=== Applying configuration to node $ip ==="
  talosctl apply-config --insecure \
    --nodes $ip \
    --file controlplane.yaml
  echo "Configuration applied to $ip"
  echo ""
done
```


2. Run this command to apply the `worker.yaml`configuration to your worker node:

```bash
for ip in "${WORKER_IP[@]}"; do
  echo "=== Applying configuration to node $ip ==="
  talosctl apply-config --insecure \
    --nodes $ip \
    --file worker.yaml
  echo "Configuration applied to $ip"
  echo ""
done
```

## Step 11: Manage Your Talos Configuration File 

The `talosconfig` is your key to managing the Talos Linux cluster, without it, you cannot authenticate or communicate with your cluster nodes using `talosctl`.

You have two options for managing your `talosconfig`:

1.  Merge your new `talosconfig` into the default configuration file located at `~/.talos/config`:

```bash
talosctl config merge ./talosconfig
```


2. Copy the configuration file to your `~/.talos` directory and set the `TALOSCONFIG` environment variable:

```bash
mkdir -p ~/.talos
cp ./talosconfig ~/.talos/config
export TALOSCONFIG=~/.talos/config
```


## Step 12: Set Endpoints of Your Control Plane Nodes

Configure your endpoints to enable talosctl to automatically load balance requests and fail over between control plane nodes when individual nodes become unavailable.


Run this command to configure your endpoints. Replace the placeholders `<control_plane_IP_1> <control_plane_IP_2> <control_plane_IP_3>` with the IP addresses of your control plane nodes:

```bash
talosctl config endpoint <control_plane_IP_1> <control_plane_IP_2> <control_plane_IP_3>
```
**For example**:

If your control plane nodes IP addresses are `192.168.0.2`, `192.168.0.3`, `192.168.0.4`, your command would be:

```bash
talosctl config endpoint 192.168.0.2 192.168.0.3 192.168.0.4
```

## Step 13: Bootstrap Your Kubernetes Cluster

Wait for your control plane nodes to finish booting, then bootstrap your etcd cluster by running the command below.

Replace the `<control-plane-IP>` placeholder with the IP address of ONE of your three control plane nodes:

```bash
talosctl bootstrap --nodes <control-plane-IP>
```

**Note**: Run this command ONCE on a SINGLE control plane node. If you have multiple control plane nodes, you can choose any of them.

## Step 14: Get Kubernetes Access

Download your `kubeconfig` file to start using `kubectl` with your cluster. These commands must be run against a single control plane node.

You have two options for managing your `kubeconfig`. Replace `<control-plane-IP>` with the IP address of any one of your control plane nodes:


- Merge into your default `kubeconfig`:

```bash
talosctl kubeconfig --nodes <control-plane-IP>
```

- Create a separate `kubeconfig` file:

```bash
talosctl kubeconfig alternative-kubeconfig --nodes <control-plane-IP>
export KUBECONFIG=./alternative-kubeconfig

```

## Step 15: Verify Your Nodes Are Running

Run the command to ensure that your nodes are running:

```bash
kubectl get nodes
```

## Next Steps

Congratulations! You now have a working production grade Talos Linux Kubernetes cluster. 


### What's Next?
- Deploy an application
- Set up persistent storage
- Configure networking policies
- Explore the talosctl CLI reference.

