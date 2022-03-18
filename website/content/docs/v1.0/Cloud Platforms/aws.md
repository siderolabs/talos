---
title: "AWS"
description: "Creating a cluster via the AWS CLI."
---

## Official AMI Images

Official AMI image ID can be found in the `cloud-images.json` file attached to the Talos release:

```bash
curl -sL https://github.com/talos-systems/talos/releases/download/v0.15.0/cloud-images.json | \
    jq -r '.[] | select(.region == "us-east-1") | select (.arch == "amd64") | .id'
```

Replace `us-east-1` and `amd64` in the line above with the desired region and architecture.

## Creating a Cluster via the AWS CLI

In this guide we will create an HA Kubernetes cluster with 3 worker nodes.
We assume an existing VPC, and some familiarity with AWS.
If you need more information on AWS specifics, please see the [official AWS documentation](https://docs.aws.amazon.com).

### Create the Subnet

```bash
aws ec2 create-subnet \
    --region $REGION \
    --vpc-id $VPC \
    --cidr-block ${CIDR_BLOCK}
```

### Create the AMI

#### Prepare the Import Prerequisites

##### Create the S3 Bucket

```bash
aws s3api create-bucket \
    --bucket $BUCKET \
    --create-bucket-configuration LocationConstraint=$REGION \
    --acl private
```

##### Create the `vmimport` Role

In order to create an AMI, ensure that the `vmimport` role exists as described in the [official AWS documentation](https://docs.aws.amazon.com/vm-import/latest/userguide/vmie_prereqs.html#vmimport-role).

Note that the role should be associated with the S3 bucket we created above.

##### Create the Image Snapshot

First, download the AWS image from a Talos release:

```bash
curl -LO https://github.com/talos-systems/talos/releases/latest/download/aws-amd64.tar.gz | tar -xv
```

Copy the RAW disk to S3 and import it as a snapshot:

```bash
aws s3 cp disk.raw s3://$BUCKET/talos-aws-tutorial.raw
aws ec2 import-snapshot \
    --region $REGION \
    --description "Talos kubernetes tutorial" \
    --disk-container "Format=raw,UserBucket={S3Bucket=$BUCKET,S3Key=talos-aws-tutorial.raw}"
```

Save the `SnapshotId`, as we will need it once the import is done.
To check on the status of the import, run:

```bash
aws ec2 describe-import-snapshot-tasks \
    --region $REGION \
    --import-task-ids
```

Once the `SnapshotTaskDetail.Status` indicates `completed`, we can register the image.

##### Register the Image

```bash
aws ec2 register-image \
    --region $REGION \
    --block-device-mappings "DeviceName=/dev/xvda,VirtualName=talos,Ebs={DeleteOnTermination=true,SnapshotId=$SNAPSHOT,VolumeSize=4,VolumeType=gp2}" \
    --root-device-name /dev/xvda \
    --virtualization-type hvm \
    --architecture x86_64 \
    --ena-support \
    --name talos-aws-tutorial-ami
```

We now have an AMI we can use to create our cluster.
Save the AMI ID, as we will need it when we create EC2 instances.

### Create a Security Group

```bash
aws ec2 create-security-group \
    --region $REGION \
    --group-name talos-aws-tutorial-sg \
    --description "Security Group for EC2 instances to allow ports required by Talos"
```

Using the security group ID from above, allow all internal traffic within the same security group:

```bash
aws ec2 authorize-security-group-ingress \
    --region $REGION \
    --group-name talos-aws-tutorial-sg \
    --protocol all \
    --port 0 \
    --source-group $SECURITY_GROUP
```

and expose the Talos and Kubernetes APIs:

```bash
aws ec2 authorize-security-group-ingress \
    --region $REGION \
    --group-name talos-aws-tutorial-sg \
    --protocol tcp \
    --port 6443 \
    --cidr 0.0.0.0/0

aws ec2 authorize-security-group-ingress \
    --region $REGION \
    --group-name talos-aws-tutorial-sg \
    --protocol tcp \
    --port 50000-50001 \
    --cidr 0.0.0.0/0
```

### Create a Load Balancer

```bash
aws elbv2 create-load-balancer \
    --region $REGION \
    --name talos-aws-tutorial-lb \
    --type network --subnets $SUBNET
```

Take note of the DNS name and ARN.
We will need these soon.

### Create the Machine Configuration Files

#### Generating Base Configurations

Using the DNS name of the loadbalancer created earlier, generate the base configuration files for the Talos machines:

```bash
$ talosctl gen config talos-k8s-aws-tutorial https://<load balancer IP or DNS>:<port> --with-examples=false --with-docs=false
created controlplane.yaml
created worker.yaml
created talosconfig
```

Take note that the generated configs are too long for AWS userdata field if the `--with-examples` and `--with-docs` flags are not passed.

At this point, you can modify the generated configs to your liking.

Optionally, you can specify `--config-patch` with RFC6902 jsonpatch which will be applied during the config generation.

#### Validate the Configuration Files

```bash
$ talosctl validate --config controlplane.yaml --mode cloud
controlplane.yaml is valid for cloud mode
$ talosctl validate --config worker.yaml --mode cloud
worker.yaml is valid for cloud mode
```

### Create the EC2 Instances

> Note: There is a known issue that prevents Talos from running on T2 instance types.
> Please use T3 if you need burstable instance types.

#### Create the Control Plane Nodes

```bash
CP_COUNT=1
while [[ "$CP_COUNT" -lt 4 ]]; do
  aws ec2 run-instances \
    --region $REGION \
    --image-id $AMI \
    --count 1 \
    --instance-type t3.small \
    --user-data file://controlplane.yaml \
    --subnet-id $SUBNET \
    --security-group-ids $SECURITY_GROUP \
    --associate-public-ip-address \
    --tag-specifications "ResourceType=instance,Tags=[{Key=Name,Value=talos-aws-tutorial-cp-$CP_COUNT}]"
  ((CP_COUNT++))
done
```

> Make a note of the resulting `PrivateIpAddress` from the init and controlplane nodes for later use.

#### Create the Worker Nodes

```bash
aws ec2 run-instances \
    --region $REGION \
    --image-id $AMI \
    --count 3 \
    --instance-type t3.small \
    --user-data file://worker.yaml \
    --subnet-id $SUBNET \
    --security-group-ids $SECURITY_GROUP
    --tag-specifications "ResourceType=instance,Tags=[{Key=Name,Value=talos-aws-tutorial-worker}]"
```

### Configure the Load Balancer

```bash
aws elbv2 create-target-group \
    --region $REGION \
    --name talos-aws-tutorial-tg \
    --protocol TCP \
    --port 6443 \
    --target-type ip \
    --vpc-id $VPC
```

Now, using the target group's ARN, and the **PrivateIpAddress** from the instances that you created :

```bash
aws elbv2 register-targets \
    --region $REGION \
    --target-group-arn $TARGET_GROUP_ARN \
    --targets Id=$CP_NODE_1_IP  Id=$CP_NODE_2_IP  Id=$CP_NODE_3_IP
```

Using the ARNs of the load balancer and target group from previous steps, create the listener:

```bash
aws elbv2 create-listener \
    --region $REGION \
    --load-balancer-arn $LOAD_BALANCER_ARN \
    --protocol TCP \
    --port 443 \
    --default-actions Type=forward,TargetGroupArn=$TARGET_GROUP_ARN
```

### Bootstrap Etcd

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
