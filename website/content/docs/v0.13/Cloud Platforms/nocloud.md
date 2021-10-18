---
title: "Nocloud"
description: "Creating a cluster via the CLI using qemu."
---

Talos supports [nocloud](https://cloudinit.readthedocs.io/en/latest/topics/datasources/nocloud.html) data source implementation.
There are two ways to configure your server:

* SMBIOS “serial number” option
* CDROM or USB-flash filesystem

### SMBIOS

This method works only with network.
DHCP and HTTP(s) server required.

```
ds=nocloud-net;s=http://10.10.0.1/configs/;h=HOSTNAME
```

After the network initialization is complete, Talos fetches:

* the machine config from http://10.10.0.1/configs/user-data
* the network config (if it exists) from http://10.10.0.1/configs/network-config

#### QEMU

Simply add the following flag when starting the VM.

```
-smbios type=1,serial=ds=nocloud-net;s=http://10.10.0.1/configs/
```

#### Proxmox

Proxmox VM config /etc/pve/qemu-server/$ID.conf

```conf
...
smbios1: uuid=ceae4d10,serial=ZHM9bm9jbG91ZC1uZXQ7cz1odHRwOi8vMTAuMTAuMC4xL2NvbmZpZ3Mv,base64=1
...
```

Where serial is a base64-encoded string of `ds=nocloud-net;s=http://10.10.0.1/configs/`.
You can use also use the Proxmox GUI to configure this information.

### CDROM/USB

Talos can also get machine config from local storage without running a network services (DHCP/HTTP).

You can provide configs to the server via files on a vfat or iso9660 filesystem. The filesystem volume label must be ```cidata``` or ```CIDATA```.

#### QEMU

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

```
qemu-system-x86_64 \
    ...
    -cdrom iso/cidata.iso \
    ...
```

#### Proxmox

Proxmox can create cloud-init disk for you.
Edit the config file as follows, substituting your own infomation as necessary:

```config
cicustom: user=local:snippets/master-1.yml
ipconfig0: ip=192.168.1.10/24,gw=192.168.10.254
nameserver: 1.1.1.1
searchdomain: local
```

Where `snippets/master-1.yml` is Talos machine config.
It usualy located at `/var/lib/vz/snippets/master-1.yml`.
This file must be placed in the path manually, as Proxmox does not support uploading of snippets through GUI.
