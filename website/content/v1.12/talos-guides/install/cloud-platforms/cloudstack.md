---
title: "CloudStack"
description: "Creating a cluster via the CLI (cmk) on Apache CloudStack."
---

## Creating a Talos Linux Cluster on Apache CloudStack via the CMK CLI

In this guide we will create an single node Kubernetes cluster in Apache CloudStack.

We assume Apache CloudStack is already running in a basic configuration - and some familiarity with Apache CloudStack.

We will be using the [CloudStack Cloudmonkey](https://github.com/apache/cloudstack-cloudmonkey) CLI tool.

Please see the [official Apache CloudStack documentation](https://docs.cloudstack.apache.org/en/latest/) for information related to Apache CloudStack.

### Obtain the Talos Image

Download the Talos CloudStack image `cloudstack-amd64.raw.gz` from the [Image Factory](https://factory.talos.dev).

> Note: the minimum version of Talos required to support Apache CloudStack is v1.8.0.

Using an upload method of your choice, upload the image to a Apache CloudStack.

You might be able to use the "Register Template from URL" to download the image directly from the Image Factory.

> Note: CloudStack does not seem to like compressed images, so you might have to download the image to a local webserver, uncompress it and let CloudStack fetch the image from there instead.
> Alternatively, you can try to remove `.gz` from URL to fetch an uncompressed image from the Image Factory.

### Get Required Variables

Next we will get a number of required variables and export them for later use:

#### Get Image Template ID

```bash
$ cmk list templates templatefilter=self | jq -r '.template[] | [.id, .name] | @tsv' | sort -k2
01813d29-1253-4080-8d29-d405d94148af   Talos 1.8.0
...
$ export IMAGE_ID=01813d29-1253-4080-8d29-d405d94148af
```

#### Get Zone ID

Get a list of Zones and select the relevant zone

```bash
$ cmk list zones | jq -r '.zone[] | [.id, .name] | @tsv' | sort -k2
a8c71a6f-2e09-41ed-8754-2d4dd8783920  fsn1
9d38497b-d810-42ab-a772-e596994d21d2  fsn2
...
$ export ZONE_ID=a8c71a6f-2e09-41ed-8754-2d4dd8783920
```

#### Get Service Offering ID

Get a list of service offerings (instance types) and select the desired offering

```bash
$ cmk list serviceofferings | jq -r '.serviceoffering[] | [.id, .memory, .cpunumber, .name] | @tsv' | sort -k4
82ac8c87-22ee-4ec3-8003-c80b09efe02c  2048  2 K8S-CP-S
c7f5253e-e1f1-4e33-a45e-eb2ebbc65fd4  4096  2 K8S-WRK-S
...
$ export SERVICEOFFERING_ID=82ac8c87-22ee-4ec3-8003-c80b09efe02c
```

#### Get Network ID

Get a list of networks and select the relevant network for your cluster.

```bash
$ cmk list networks zoneid=${ZONE_ID} | jq -r '.network[] | [.id, .type, .name] | @tsv' | sort -k3
f706984f-9dd1-4cb8-9493-3fba1f0de7e3  Isolate  demo
143ed8f1-3cc5-4ba2-8717-457ad993cf25  Isolated  talos
...
$ export NETWORK_ID=143ed8f1-3cc5-4ba2-8717-457ad993cf25
```

#### Get next free Public IP address and ID

To create a loadbalancer for the K8S API Endpoint, find the next available public IP address in the zone.

(In this test environment, the 10.0.0.0/24 RFC-1918 IP range has been configured as "Public IP addresses")

```bash
$ cmk list publicipaddresses zoneid=${ZONE_ID} state=free forvirtualnetwork=true | jq -r '.publicipaddress[] | [.id, .ipaddress] | @tsv' | sort -k2
1901d946-3797-48aa-a113-8fb730b0770a  10.0.0.102
fa207d0e-c8f8-4f09-80f0-d45a6aac77eb  10.0.0.103
aa397291-f5dc-4903-b299-277161b406cb  10.0.0.104
...
$ export PUBLIC_IPADDRESS=10.0.0.102
$ export PUBLIC_IPADDRESS_ID=1901d946-3797-48aa-a113-8fb730b0770a
```

#### Acquire and Associate Public IP Address

Acquire and associate the public IP address with the network we selected earlier.

```bash
$ cmk associateIpAddress ipaddress=${PUBLIC_IPADDRESS} networkid=${NETWORK_ID}
{
  "ipaddress": {
    ...,
    "ipaddress": "10.0.0.102",
    ...
  }
}
```

#### Create LB and FW rule using the Public IP Address

Create a Loadbalancer for the K8S API Endpoint.

> Note: The "create loadbalancerrule" also takes care of creating a corresponding firewallrule.

```bash
$ cmk create loadbalancerrule algorithm=roundrobin name="k8s-api" privateport=6443 publicport=6443 openfirewall=true publicipid=${PUBLIC_IPADDRESS_ID} cidrlist=0.0.0.0/0
{
  "loadbalancer": {
    ...
    "name": "k8s-api",
    "networkid": "143ed8f1-3cc5-4ba2-8717-457ad993cf25",
    "privateport": "6443",
    "publicip": "10.0.0.102",
    "publicipid": "1901d946-3797-48aa-a113-8fb730b0770a",
    "publicport": "6443",
    ...
  }
}
```

### Create the Talos Configuration Files

Finally it's time to generate the Talos configuration files, using the Public IP address assigned to the loadbalancer.

```bash
$ talosctl gen config talos-cloudstack https://${PUBLIC_IPADDRESS}:6443 --with-docs=false --with-examples=false
created controlplane.yaml
created worker.yaml
created talosconfig
```

Make any adjustments to the `controlplane.yaml` and/or `worker.yaml` as you like.

> Note: Remember to validate!

#### Create Talos VM

Next we will create the actual VM and supply the `controlplane.yaml` as base64 encoded `userdata`.

```bash
$ cmk deploy virtualmachine zoneid=${ZONE_ID} templateid=${IMAGE_ID} serviceofferingid=${SERVICEOFFERING_ID} networkIds=${NETWORK_ID} name=talosdemo  usersdata=$(base64 controlplane.yaml | tr -d '\n')
{
  "virtualmachine": {
    "account": "admin",
    "affinitygroup": [],
    "cpunumber": 2,
    "cpuspeed": 2000,
    "cpuused": "0.3%",
    ...
  }
}
```

#### Get Talos VM ID and Internal IP address

Get the ID of our newly created VM.
(Also available in the full output of the above command.)

```bash
$ cmk list virtualmachines | jq -r '.virtualmachine[] | [.id, .ipaddress, .name]|@tsv' | sort -k3
9c119627-cb38-4b64-876b-ca2b79820b5a  10.1.1.154  srv03
545099fc-ec2d-4f32-915d-b0c821cfb634  10.1.1.97   srv04
d37aeca4-7d1f-45cd-9a4d-97fdbf535aa1  10.1.1.243  talosdemo
$ export VM_ID=d37aeca4-7d1f-45cd-9a4d-97fdbf535aa1
$ export VM_IP=10.1.1.243
```

#### Get Load Balancer ID

Obtain the ID of the `loadbalancerrule` we created earlier.

```bash
$ cmk list loadbalancerrules | jq -r '.loadbalancerrule[]| [.id, .publicip, .name] | @tsv' | sort -k2
ede6b711-b6bc-4ade-9e48-4b3f5aa59934  10.0.0.102  k8s-api
1bad3c46-96fa-4f50-a4fc-9a46a54bc350  10.0.0.197  ac0b5d98cf6a24d55a4fb2f9e240c473-tcp-443
$ export LB_RULE_ID=ede6b711-b6bc-4ade-9e48-4b3f5aa59934
```

#### Assign Talos VM to Load Balancer

With the ID of the VM and the load balancer, we can assign the VM to the `loadbalancerrule`, making the K8S API endpoint available via the Load Balancer

```bash
cmk assigntoloadbalancerrule id=${LB_RULE_ID} virtualmachineids=${VM_ID}
```

### Bootstrap Etcd

Once the Talos VM has booted, it time to bootstrap etcd.

Configure `talosctl` with IP addresses of the control plane node's IP address.

Set the `endpoints` and `nodes`:

```bash
talosctl --talosconfig talosconfig config endpoint ${VM_IP}
talosctl --talosconfig talosconfig config node ${VM_IP}
```

Next, bootstrap `etcd`:

```bash
talosctl --talosconfig talosconfig bootstrap
```

### Retrieve the `kubeconfig`

At this point we can retrieve the admin `kubeconfig` by running:

```bash
talosctl --talosconfig talosconfig kubeconfig .
```

We can also watch the cluster bootstrap via:

```bash
talosctl --talosconfig talosconfig dashboard
```
