---
title: "Oracle"
description: "Creating a cluster via the CLI (oci) on OracleCloud.com."
---

## Upload image

Oracle Cloud at the moment does not have a Talos official image.
So you can use [Bring Your Own Image (BYOI)](https://docs.oracle.com/en-us/iaas/Content/Compute/References/bringyourownimage.htm) approach.

Once the image is uploaded, set the ```Boot volume type``` to ``Paravirtualized`` mode.

OracleCloud has highly available NTP service, it can be enabled in Talos machine config with:

```yaml
machine:
  time:
    servers:
      - 169.254.169.254
```

## Creating a Cluster via the CLI

Login to the [console](https://www.oracle.com/cloud/).
And open the Cloud Shell.

### Create a network

```bash
export cidr_block=10.0.0.0/16
export subnet_block=10.0.0.0/24
export compartment_id=<substitute-value-of-compartment_id> # https://docs.cloud.oracle.com/en-us/iaas/tools/oci-cli/latest/oci_cli_docs/cmdref/network/vcn/create.html#cmdoption-compartment-id

export vcn_id=$(oci network vcn create --cidr-block $cidr_block --display-name talos-example --compartment-id $compartment_id --query data.id --raw-output)
export rt_id=$(oci network subnet create --cidr-block $subnet_block --display-name kubernetes --compartment-id $compartment_id --vcn-id $vcn_id --query data.route-table-id --raw-output)
export ig_id=$(oci network internet-gateway create --compartment-id $compartment_id --is-enabled true --vcn-id $vcn_id --query data.id --raw-output)

oci network route-table update --rt-id $rt_id --route-rules "[{\"cidrBlock\":\"0.0.0.0/0\",\"networkEntityId\":\"$ig_id\"}]" --force

# disable firewall
export sl_id=$(oci network vcn list --compartment-id $compartment_id --query 'data[0]."default-security-list-id"' --raw-output)

oci network security-list update --security-list-id $sl_id --egress-security-rules '[{"destination": "0.0.0.0/0", "protocol": "all", "isStateless": false}]' --ingress-security-rules '[{"source": "0.0.0.0/0", "protocol": "all", "isStateless": false}]' --force
```

### Create a Load Balancer

Create a load balancer by issuing the commands shown below.
Save the IP/DNS name, as this info will be used in the next step.

```bash
export subnet_id=$(oci network subnet list --compartment-id=$compartment_id --display-name kubernetes --query data[0].id --raw-output)
export network_load_balancer_id=$(oci nlb network-load-balancer create --compartment-id $compartment_id --display-name controlplane-lb --subnet-id $subnet_id --is-preserve-source-destination false --is-private false --query data.id --raw-output)

cat <<EOF > talos-health-checker.json
{
  "intervalInMillis": 10000,
  "port": 50000,
  "protocol": "TCP"
}
EOF

oci nlb backend-set create --health-checker file://talos-health-checker.json --name talos --network-load-balancer-id $network_load_balancer_id --policy TWO_TUPLE --is-preserve-source false
oci nlb listener create --default-backend-set-name talos --name talos --network-load-balancer-id $network_load_balancer_id --port 50000 --protocol TCP

cat <<EOF > controlplane-health-checker.json
{
  "intervalInMillis": 10000,
  "port": 6443,
  "protocol": "HTTPS",
  "returnCode": 200,
  "urlPath": "/readyz"
}
EOF

oci nlb backend-set create --health-checker file://controlplane-health-checker.json --name controlplane --network-load-balancer-id $network_load_balancer_id --policy TWO_TUPLE --is-preserve-source false
oci nlb listener create --default-backend-set-name controlplane --name controlplane --network-load-balancer-id $network_load_balancer_id --port 6443 --protocol TCP

# Save the external IP
oci nlb network-load-balancer list --compartment-id $compartment_id --display-name controlplane-lb --query 'data.items[0]."ip-addresses"'
```

### Create the Machine Configuration Files

#### Generating Base Configurations

Using the IP/DNS name of the loadbalancer created earlier, generate the base configuration files for the Talos machines by issuing:

```bash
$ talosctl gen config talos-k8s-oracle-tutorial https://<load balancer IP or DNS>:6443 --additional-sans <load balancer IP or DNS>
created controlplane.yaml
created worker.yaml
created talosconfig
```

At this point, you can modify the generated configs to your liking.
Optionally, you can specify `--config-patch` with RFC6902 jsonpatches which will be applied during the config generation.

#### Validate the Configuration Files

Validate any edited machine configs with:

```bash
$ talosctl validate --config controlplane.yaml --mode cloud
controlplane.yaml is valid for cloud mode
$ talosctl validate --config worker.yaml --mode cloud
worker.yaml is valid for cloud mode
```

### Create the Servers

#### Create the Control Plane Nodes

Create the control plane nodes with:

```bash
export shape='VM.Standard.A1.Flex'
export subnet_id=$(oci network subnet list --compartment-id=$compartment_id --display-name kubernetes --query data[0].id --raw-output)
export image_id=$(oci compute image list --compartment-id $compartment_id --shape $shape --operating-system Talos --limit 1 --query data[0].id --raw-output)
export availability_domain=$(oci iam availability-domain list --compartment-id=$compartment_id --query data[0].name --raw-output)
export network_load_balancer_id=$(oci nlb network-load-balancer list --compartment-id $compartment_id --display-name controlplane-lb --query 'data.items[0].id' --raw-output)

cat <<EOF > shape.json
{
  "memoryInGBs": 4,
  "ocpus": 1
}
EOF

export instance_id=$(oci compute instance launch --shape $shape --shape-config file://shape.json --availability-domain $availability_domain --compartment-id $compartment_id --image-id $image_id --subnet-id $subnet_id --display-name controlplane-1 --private-ip 10.0.0.11 --assign-public-ip true --launch-options '{"networkType":"PARAVIRTUALIZED"}' --user-data-file controlplane.yaml --query 'data.id' --raw-output)

oci nlb backend create --backend-set-name talos --network-load-balancer-id $network_load_balancer_id --port 50000 --target-id $instance_id
oci nlb backend create --backend-set-name controlplane --network-load-balancer-id $network_load_balancer_id --port 6443 --target-id $instance_id

export instance_id=$(oci compute instance launch --shape $shape --shape-config file://shape.json --availability-domain $availability_domain --compartment-id $compartment_id --image-id $image_id --subnet-id $subnet_id --display-name controlplane-2 --private-ip 10.0.0.12 --assign-public-ip true --launch-options '{"networkType":"PARAVIRTUALIZED"}' --user-data-file controlplane.yaml --query 'data.id' --raw-output)

oci nlb backend create --backend-set-name talos --network-load-balancer-id $network_load_balancer_id --port 50000 --target-id $instance_id
oci nlb backend create --backend-set-name controlplane --network-load-balancer-id $network_load_balancer_id --port 6443 --target-id $instance_id

export instance_id=$(oci compute instance launch --shape $shape --shape-config file://shape.json --availability-domain $availability_domain --compartment-id $compartment_id --image-id $image_id --subnet-id $subnet_id --display-name controlplane-3 --private-ip 10.0.0.13 --assign-public-ip true --launch-options '{"networkType":"PARAVIRTUALIZED"}' --user-data-file controlplane.yaml --query 'data.id' --raw-output)

oci nlb backend create --backend-set-name talos --network-load-balancer-id $network_load_balancer_id --port 50000 --target-id $instance_id
oci nlb backend create --backend-set-name controlplane --network-load-balancer-id $network_load_balancer_id --port 6443 --target-id $instance_id
```

#### Create the Worker Nodes

Create the worker nodes with the following command, repeating (and incrementing the name counter) as many times as desired.

```bash
export subnet_id=$(oci network subnet list --compartment-id=$compartment_id --display-name kubernetes --query data[0].id --raw-output)
export image_id=$(oci compute image list --compartment-id $compartment_id --operating-system Talos --limit 1 --query data[0].id --raw-output)
export availability_domain=$(oci iam availability-domain list --compartment-id=$compartment_id --query data[0].name --raw-output)
export shape='VM.Standard.E2.1.Micro'

oci compute instance launch --shape $shape --availability-domain $availability_domain --compartment-id $compartment_id --image-id $image_id --subnet-id $subnet_id --display-name worker-1 --assign-public-ip true --user-data-file worker.yaml

oci compute instance launch --shape $shape --availability-domain $availability_domain --compartment-id $compartment_id --image-id $image_id --subnet-id $subnet_id --display-name worker-2 --assign-public-ip true --user-data-file worker.yaml

oci compute instance launch --shape $shape --availability-domain $availability_domain --compartment-id $compartment_id --image-id $image_id --subnet-id $subnet_id --display-name worker-3 --assign-public-ip true --user-data-file worker.yaml
```

### Bootstrap Etcd

To configure `talosctl` we will need the first control plane node's IP.
This can be found by issuing:

```bash
export instance_id=$(oci compute instance list --compartment-id $compartment_id --display-name controlplane-1 --query 'data[0].id' --raw-output)

oci compute instance list-vnics --instance-id $instance_id --query 'data[0]."private-ip"' --raw-output
```

Set the `endpoints` and `nodes` for your talosconfig with:

```bash
talosctl --talosconfig talosconfig config endpoint <load balancer IP or DNS>
talosctl --talosconfig talosconfig config node <control-plane-1-IP>
```

Bootstrap `etcd` on the first control plane node with:

```bash
talosctl --talosconfig talosconfig bootstrap
```

### Retrieve the `kubeconfig`

At this point we can retrieve the admin `kubeconfig` by running:

```bash
talosctl --talosconfig talosconfig kubeconfig .
```
