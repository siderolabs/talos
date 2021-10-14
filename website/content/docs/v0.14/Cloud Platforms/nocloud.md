---
title: "Nocloud"
description: "Creating a cluster via the CLI using qemu."
---

Talos supports [nocloud](https://cloudinit.readthedocs.io/en/latest/topics/datasources/nocloud.html) data source implementation.

There are two ways to configure Talos server with `nocloud` platform:

* via SMBIOS "serial number" option
* using CDROM or USB-flash filesystem

### SMBIOS Serial Number

This method requires the network connection to be up (e.g. via DHCP).
Configuration is delivered from the HTTP server.

```text
ds=nocloud-net;s=http://10.10.0.1/configs/;h=HOSTNAME
```

After the network initialization is complete, Talos fetches:

* the machine config from `http://10.10.0.1/configs/user-data`
* the network config (if available) from `http://10.10.0.1/configs/network-config`

#### SMBIOS: QEMU

Add the following flag to `qemu` command line when starting a VM:

```bash
qemu-system-x86_64 \
  ...\
  -smbios type=1,serial=ds=nocloud-net;s=http://10.10.0.1/configs/
```

#### SMBIOS: Proxmox

Set the source machine config through the serial number on Proxmox GUI.

<img src="/images/no-cloud/proxmox-smbios.png" width="920px">

The Proxmox stores the VM config at `/etc/pve/qemu-server/$ID.conf` (```$ID``` - VM ID number of virtual machine), you will see something like:

```conf
...
smbios1: uuid=ceae4d10,serial=ZHM9bm9jbG91ZC1uZXQ7cz1odHRwOi8vMTAuMTAuMC4xL2NvbmZpZ3Mv,base64=1
...
```

Where serial holds the base64-encoded string version of `ds=nocloud-net;s=http://10.10.0.1/configs/`.

### CDROM/USB

Talos can also get machine config from local attached storage without any prior network connection being established.

You can provide configs to the server via files on a VFAT or ISO9660 filesystem.
The filesystem volume label must be ```cidata``` or ```CIDATA```.

#### Example: QEMU

Create and prepare Talos machine config:

```bash
export CONTROL_PLANE_IP=192.168.1.10

talosctl gen config talos-nocloud https://$CONTROL_PLANE_IP:6443 --output-dir _out
```

Prepare cloud-init configs:

```bash
mkdir -p iso
mv _out/controlplane.yaml iso/user-data
echo "local-hostname: controlplane-1" > iso/meta-data
cat > iso/network-config << EOF
version: 1
config:
   - type: physical
     name: eth0
     mac_address: "52:54:00:12:34:00"
     subnets:
        - type: static
          address: 192.168.1.10
          netmask: 255.255.255.0
          gateway: 192.168.1.254
EOF
```

Create cloud-init iso image

```bash
cd iso && genisoimage -output cidata.iso -V cidata -r -J user-data meta-data network-config
```

Start the VM

```bash
qemu-system-x86_64 \
    ...
    -cdrom iso/cidata.iso \
    ...
```

#### Example: Proxmox

Proxmox can create cloud-init disk [for you](https://pve.proxmox.com/wiki/Cloud-Init_Support).
Edit the cloud-init config information in Proxmox as follows, substitute your own information as necessary:

<img src="/images/no-cloud/proxmox-cloudinit.png" width="600px">

and then update ```cicustom``` param at `/etc/pve/qemu-server/$ID.conf`.

```config
cicustom: user=local:snippets/master-1.yml
ipconfig0: ip=192.168.1.10/24,gw=192.168.10.254
nameserver: 1.1.1.1
searchdomain: local
```

> Note: `snippets/master-1.yml` is Talos machine config.
It is usually located at `/var/lib/vz/snippets/master-1.yml`.
This file must be placed to this path manually, as Proxmox does not support snippet uploading via API/GUI.

Click on `Regenerate Image` button after the above changes are made.
