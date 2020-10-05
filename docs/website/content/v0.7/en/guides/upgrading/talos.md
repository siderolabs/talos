---
title: Upgrading alos
---

Keeping the infrastructure up to date is important for a stable and secure environment.
Talos makes the upgrade process simple and efficient.
There are 2 ways of upgrading Talos in this guide.

### Method 1: upgrade trough talosctl

We have a dedicated upgrade command in the `talosctl` CLI.
There are some other settings worth checking before upgrading.

Let's start by verifying the current version.

``` bash
$ talosctl version
Client:
        Tag:         v0.6.0-beta.0
        SHA:         d17eb3d4
        Built:
        Go version:  go1.14.6
        OS/Arch:     linux/amd64

Server:
        NODE:        192.168.1.82
        Tag:         v0.6.0-alpha.6
        SHA:         568e3985
        Built:
        Go version:  go1.14.6
        OS/Arch:     linux/amd64
```

As you can see in the above example, we're running v0.6.0-alpha.6.
We're going to upgrade to the v0.6.0-beta.0 today using the `talosctl upgrade` command.
As you can see in the example above, we're going to upgrade the node: 192.168.1.82.

Please make sure you're connected to the right node, otherwise do a: `talosctl config node <IP or DNS>`.

> To get the right image, you can checkout the [release page on github](https://github.com/talos-systems/talos/releases/)
> See [additional information](#getting-the-right-version) for more info on getting the correct image.

``` bash
$ talosctl upgrade -i docker.io/autonomy/installer:v0.6.0-beta.0 -p
NODE           ACK                        STARTED
192.168.1.82   Upgrade request received   2020-08-07 19:18:30.60710213 +0200 CEST m=+19.917367174
```

> The description of the flags used:
> -i, --image string   the container image to use for performing the install
> -p, --preserve       preserve data

After the command has been run, it can take a while before it responds.
Do not abort it!
This will kill the proccess and can leave your system in a unresponsive state.

The machine now pulls in the new image, and start the update process.
To learn more about the full process please scroll down to the [additional information](#boot-process) section below.

In about ~5 min (depending on your hardware) the machine will come back online.
Verify the machine is working by running `talosctl version`

> If you're waiting on the process, you can also use `watch -n 5 talosctl version`

If the command is succesful, you will see that it has the new version installed:

``` bash
$ talosctl version
Client:
        Tag:         v0.6.0-beta.0
        SHA:         d17eb3d4
        Built:
        Go version:  go1.14.6
        OS/Arch:     linux/amd64

Server:
        NODE:        192.168.1.82
        Tag:         v0.6.0-beta.0
        SHA:         d17eb3d4
        Built:
        Go version:  go1.14.6
        OS/Arch:     linux/amd64
```

To verify Kubelet is updated as well, we need to run the `kubectl` command:

```bash
$ kubectl get nodes -o wide
NAME      STATUS   ROLES    AGE   VERSION          INTERNAL-IP    EXTERNAL-IP   OS-IMAGE                 KERNEL-VERSION   CONTAINER-RUNTIME
control   Ready    master   32h   v1.19.0-beta.1   192.168.1.81   <none>        Talos (v0.6.0-alpha.6)   5.7.7-talos      containerd://1.3.6
worker1   Ready    master   32h   v1.19.0-rc.3     192.168.1.82   <none>        Talos (v0.6.0-beta.0)    5.7.7-talos      containerd://1.3.6
worker2   Ready    master   32h   v1.19.0-rc.3     192.168.1.83   <none>        Talos (v0.6.0-beta.0)    5.7.7-talos      containerd://1.3.6
```

As you can see worker1 and worker2 are running the new beta, and the control node is running alpha.6 with the v1.19.0-beta.1 of kubelet.
That's it!
The upgrade was succesful.

### Method 2: upgrade trough iPXE

There is another way to upgrade Talos, and that's trough the iPXE server we used to intially install this cluster.
If you don't have a PXE server, then please use Method 1 to update.

In the example below we're using Matchbox as our PXE server.

#### Requirements

This section has some requirements, these are listed below:

- Working PXE server
- `persist: false` set in the configuration
- Access to PXE server + metadata server

#### Preparations

Before starting with the upgrade, make sure that your node boots from network(PXE) all the time!
Otherwise our changes won't make it to the node, since we're doing the changes on our PXE server.

#### Upgrading

To get started, we're going to upgrade our PXE boot files first.
For matchbox these are:

- initramfs.xz
- vmlinuz

This might differ from your PXE setup, please use the correct files.

``` bash
wget https://github.com/talos-systems/talos/releases/download/v0.6.0-beta.0/initramfs.xz
wget https://github.com/talos-systems/talos/releases/download/v0.6.0-beta.0/vmlinuz
```

Once they are in the right place, It's time to make sure the Kubelet get's updated as well.
In my case the kubelet updated to version `v1.19.0-rc.3`, so we need to adjust the kubelet image in the `controlplane.yaml`, `init.yaml` and `join.yaml`.

``` yaml
machine:
.....
  kubelet:
    image: docker.io/autonomy/kubelet:v1.19.0-rc.3
.....
```

> If you want to make sure new nodes are also up to date, please see [additional information](#updates)
>
> The above only updates running nodes, not any additional nodes you're adding.

#### Rebooting

If all steps are done succesfully, it's time to reboot the node you'd like to update.
Connect to the node:

``` bash
talosctl config node 192.168.1.81
talosctl reboot
```

This will reboot the node, and reboot to the correct talosversion specified in the PXE server.

> If you're waiting on the process, you can also use `watch -n 5 talosctl version`

``` bash
$ talosctl version
Client:
        Tag:         v0.6.0-beta.0
        SHA:         d17eb3d4
        Built:
        Go version:  go1.14.6
        OS/Arch:     linux/amd64

Server:
        NODE:        192.168.1.81
        Tag:         v0.6.0-beta.0
        SHA:         d17eb3d4
        Built:
        Go version:  go1.14.6
        OS/Arch:     linux/amd64
```

To verify Kubelet is updated as well, we need to run the `kubectl` command:

```bash
$ kubectl get nodes -o wide
NAME      STATUS   ROLES    AGE   VERSION          INTERNAL-IP    EXTERNAL-IP   OS-IMAGE                 KERNEL-VERSION   CONTAINER-RUNTIME
control   Ready    master   5d4h  v1.19.0-rc.3     192.168.1.81   <none>        Talos (v0.6.0-beta.0)    5.7.7-talos      containerd://1.3.6
worker1   Ready    master   32h   v1.19.0-rc.3     192.168.1.82   <none>        Talos (v0.6.0-beta.0)    5.7.7-talos      containerd://1.3.6
worker2   Ready    master   32h   v1.19.0-rc.3     192.168.1.83   <none>        Talos (v0.6.0-beta.0)    5.7.7-talos      containerd://1.3.6
```

As you can see all nodes are now upgraded, and are running the Talos beta and the latest kubelet version.

### Additional information

This section is used for in-depth information, and gives you more insight what's happening under the hood.

#### Getting the right version

Getting the right version can be simplified, make sure you have the correct `talosctl` corresponding to the version you're wanting to upgrade to.
Verify by running `talosctl version` the `Tag` in the client sections should match the *desired* version.

Next go to a temporary directory, we recommend using `/tmp/talos`
> If this directory doesn't exists, create it.
Once there, run the following command:

``` bash
$ talosctl gen config test https://127.0.0.1
generating PKI and tokens
created /tmp/talos/init.yaml
created /tmp/talos/controlplane.yaml
created /tmp/talos/join.yaml
created /tmp/talos/talosconfig
```

In the files just generated you can find all the updated versions of the `image` section.
> It's important you don't override your existing config.
> This will override Certificates and Keys, and might render your cluster unaccesible.

#### Boot process

During a upgrade using method 1, a new install is done under the hood.
This is achieved by installing the new version in a seperate boot directory.
To verify the boot directory, we can run the following command:

```bash
$ talosctl ls /boot/
NODE           NAME
192.168.1.82   .
192.168.1.82   EFI
192.168.1.82   boot-a
192.168.1.82   boot-b
192.168.1.82   config.yaml
192.168.1.82   syslinux
```

As you can see it has `boot-a` and `boot-b`.

There is a big chance it's currently booting from `boot-b`, since you just did a upgrade.

#### Updates

To make sure any new nodes are bootstrapped with the correct version, it's often best to upgrade all image versions.
Use the step above [to get the right version](#getting-the-right-version)
Once this is done, adjust all the `image` versions accordingly.
