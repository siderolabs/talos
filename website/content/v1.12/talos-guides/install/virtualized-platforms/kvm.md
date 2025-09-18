---
title: "KVM"
description: "Create a Talos Kubernetes cluster with KVM."
aliases:
  - ../../../virtualized-platforms/kvm
---

In this guide, you’ll create a Kubernetes cluster using `KVM` and `virsh` command-line utility.

## Requirements

Make sure you have the following installed and configured:

- **Kernel with KVM enabled** (`/dev/kvm` must exist)
- **QEMU**
- **virsh**: Command-line interface for managing KVM/QEMU VMs
- **kubectl**: Kubernetes CLI
- **talosctl**: CLI for interacting with Talos clusters
- **16 GB RAM**: Recommended minimum for running the VMs
- *(Optional)* `virt-manager` for a graphical interface

> **Note:** This guide assumes you are running all the commands from the following working directory.

Create a working directory for your project files:

```bash
mkdir -p ~/talos-kvm/configs
cd ~/talos-kvm
```

Download the latest `metal-amd64.iso` from the Talos [GitHub releases page](https://github.com/siderolabs/talos/releases).

## Configure the Network

Before we get started, let’s set up an isolated network for your Talos cluster.

Use the following command to write the required network configuration into a file.
{{< tabpane text=true >}}
{{% tab header="IPv4 Only" %}}

```bash
cat > my-talos-net.xml <<EOF
<network>
  <name>my-talos-net</name>
  <bridge name="talos-bridge" stp="on" delay="0"/>
  <forward mode='nat'>
    <nat/>
  </forward>
  <ip address="10.0.0.1" netmask="255.255.255.0">
    <dhcp>
      <range start="10.0.0.2" end="10.0.0.254"/>
    </dhcp>
  </ip>
</network>
EOF
```

{{% /tab %}}
{{% tab header="Dual Stack" %}}

```bash
cat > my-talos-net.xml <<EOF
<network ipv6='yes'>
  <name>my-talos-net</name>
  <bridge name="talos-bridge" stp="on" delay="0"/>
  <forward mode='nat'>
    <nat ipv6='yes'/>
  </forward>
  <ip address="10.0.0.1" netmask="255.255.255.0">
    <dhcp>
      <range start="10.0.0.2" end="10.0.0.254"/>
    </dhcp>
  </ip>
  <ip family='ipv6' address='2001:db8:b84b:5::1' prefix='64'>
    <dhcp>
      <range start='2001:db8:b84b:5::2' end='2001:db8:b84b:5::ffff'/>
    </dhcp>
  </ip>
</network>
EOF
```

{{% /tab %}}
{{< /tabpane >}}

Use the following command to generate the configurations and define the bridge:

```bash
virsh net-define my-talos-net.xml
```

Use the following command to start the network and flag it to auto start after reboot:

```bash
virsh net-start my-talos-net
virsh net-autostart my-talos-net
```

Verify the network:

```bash
virsh net-info my-talos-net
```

> **Note:** You should see the network as `Active`.

Expected output:

```bash
Name:           my-talos-net
UUID:           <UNIQUE UUID>
Active:         yes
Persistent:     yes
Autostart:      yes
Bridge:         talos-bridge
```

## Provisioning the Environment

Now that you have a dedicated network let's go ahead and provision VMs.

> **Note:** For the network interface emulation, `virtio` and `e1000` are supported.
>
> In the next command, you will use the `--network network=my-talos-net` flag to attach the VMs to the Talos network you created earlier.
>
> The values for --ram and --vcpus in this guide are suggestions. You can adjust these to match the resources available on your host machine and the needs of your workload.
>

Use the following command to create a controlplane node:

```bash
virt-install \
  --virt-type kvm \
  --name control-plane-node-1 \
  --ram 2048 \
  --vcpus 2 \
  --disk path=control-plane-node-1-disk.qcow2,bus=virtio,size=40,format=qcow2 \
  --cdrom metal-amd64.iso \
  --os-variant=linux2022 \
  --network network=my-talos-net \
  --boot hd,cdrom --noautoconsole
```

Use the following command to create a worker node:

```bash
virt-install \
  --virt-type kvm \
  --name worker-node-1 \
  --ram 4086 \
  --vcpus 2 \
  --disk path=worker-node-1-disk.qcow2,bus=virtio,size=40,format=qcow2 \
  --cdrom metal-amd64.iso \
  --os-variant=linux2022 \
  --network network=my-talos-net \
  --boot hd,cdrom --noautoconsole
```

Use the following command to verify that your VMs are in a running state:

```bash
virsh list
```

## Configure the Cluster

Now that you have your VMs provisioned it's time to configure the cluster. This step is done through `talosctl` command utility.

Use the following command to view your control plane IP address, and do the same for your worker node by adjusting the vm name:

```bash
virsh domifaddr control-plane-node-1
```

This guide is designed to help you get up and running quickly. To simplify the process, you will store the VM IP addresses in two environment variables: `CP_IP` and `NODE_IP`.

You can automate this step with the following commands:

```bash
export CP_IP=$(virsh domifaddr  control-plane-node-1 | egrep '/' | awk '{print $4}' | cut -d/ -f1)
export NODE_IP=$(virsh domifaddr worker-node-1 | egrep '/' | awk '{print $4}' | cut -d/ -f1)
```

Currently, your VMs are running Talos directly from the ISO image you downloaded. To install Talos onto the VM disks (so they can boot without the ISO), you need to specify the disk name during the configuration step.

Use the following command to get the name of your VMs disk:

```bash
talosctl  get disks --nodes $CP_IP --insecure
```

You should see a result similar to the following:

```bash
NODE   NAMESPACE   TYPE   ID      VERSION   SIZE     READ ONLY   TRANSPORT   ROTATIONAL   WWID   MODEL          SERIAL
       runtime     Disk   loop0   2         73 MB    true
       runtime     Disk   sr0     2         301 MB   false       ata                             QEMU DVD-ROM   QEMU_DVD-ROM_QM00001
       runtime     Disk   vda     2         43 GB    false       virtio      true
```

Use the following command to generate the Talos configurations:

```bash
talosctl gen config my-talos-cluster https://$CP_IP:6443 --install-disk /dev/vda -o configs/
```

Use the following command to apply the configurations to each VM:

```bash
talosctl apply-config --insecure --nodes $CP_IP --file configs/controlplane.yaml
talosctl apply-config --insecure --nodes $NODE_IP --file configs/worker.yaml
```

At this point your VMs will reboot.

## Bootstrapping the Cluster

After your VMs restart, you can bootstrap the cluster. Bootstrap simply means starting up your Kubernetes cluster for the first time.

To do this, provide the `talosconfig` file (generated earlier) to the `talosctl` command-line utility. The easiest way is to export the path to `talosconfig` as an environment variable.

Use the following command to export your talosconfig:

```bash
export TALOSCONFIG=$(realpath configs/talosconfig)
```

Before running the bootstrap, you need to tell `talosctl` where to connect:

```bash
talosctl config endpoint $CP_IP
```

> **Note:** Keep in mind that it may take a couple of seconds for the bootstraping to complete.

Use the following command to bootstrap the cluster:

```bash
talosctl -n $CP_IP bootstrap
```

At this point you should be able to see all the VMs that are participating in your cluster.

```bash
talosctl -n $CP_IP get members
```

## Accessing the cluster

To access your cluster you need to export the kubeconfig file from the Talos cluster.

Use the following command to export the kubeconfig file:

```bash
talosctl -n $CP_IP kubeconfig $PWD/configs/kubeconfig
```

Set the `KUBECONFIG` environment variable to use your new `kubeconfig` file.

```bash
export KUBECONFIG=$PWD/configs/kubeconfig
```

That's it! You can now use `kubectl` to interact with your Talos Kubernetes cluster. Check the status of your nodes with `kubectl` to verify they are ready.

```bash
kubectl get nodes -o wide
```

## Clean up

When you are finished with your cluster, you can delete the VMs and the isolated network to free up system resources.

```bash
virsh destroy <VM-NAME-HERE>
virsh undefine <VM-NAME-HERE> --remove-all-storage
```

Use the following command to delete the network:

```bash
virsh net-destroy --network my-talos-net
virsh net-undefine --network my-talos-net
```
