---
title: "AWS"
description: "Creating a cluster via the AWS CLI."
aliases:
  - ../../../cloud-platforms/aws
---

## Creating a Cluster via the AWS CLI

In this guide we will create an HA Kubernetes cluster with 3 worker nodes.
We assume an existing VPC, and some familiarity with AWS.
If you need more information on AWS specifics, please see the [official AWS documentation](https://docs.aws.amazon.com).

### Set the needed info

Change to your desired region:

```bash
REGION="us-west-2"
aws ec2 describe-vpcs --region $REGION

VPC="(the VpcId from the above command)"
```

### Create the Subnet

Use a CIDR block that is present on the VPC specified above.

```bash
aws ec2 create-subnet \
    --region $REGION \
    --vpc-id $VPC \
    --cidr-block ${CIDR_BLOCK}
```

Note the subnet ID that was returned, and assign it to a variable for ease of later use:

```bash
SUBNET="(the subnet ID of the created subnet)"
```

### Official AMI Images

Official AMI image ID can be found in the `cloud-images.json` file attached to the Talos release:

```bash
AMI=`curl -sL https://github.com/siderolabs/talos/releases/download/{{< release >}}/cloud-images.json | \
    jq -r '.[] | select(.region == "'$REGION'") | select (.arch == "amd64") | .id'`
echo $AMI

```

Replace `amd64` in the line above with the desired architecture.
Note the AMI id that is returned is assigned to an environment variable: it will be used later when booting instances.

If using the official AMIs, you can skip to [Creating the Security group]({{< relref "#create-a-security-group" >}})

### Create your own AMIs

> The use of the official Talos AMIs are recommended, but if you wish to build your own AMIs, follow the procedure below.

#### Create the S3 Bucket

```bash
aws s3api create-bucket \
    --bucket $BUCKET \
    --create-bucket-configuration LocationConstraint=$REGION \
    --acl private
```

#### Create the `vmimport` Role

In order to create an AMI, ensure that the `vmimport` role exists as described in the [official AWS documentation](https://docs.aws.amazon.com/vm-import/latest/userguide/required-permissions.html).

Note that the role should be associated with the S3 bucket we created above.

#### Create the Image Snapshot

First, download the AWS image from a Talos release:

```bash
curl -L https://github.com/siderolabs/talos/releases/download/{{< release >}}/aws-amd64.tar.gz | tar -xv
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

#### Register the Image

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

```bash
AMI="(AMI ID of the register image command)"
```

### Create a Security Group

```bash
aws ec2 create-security-group \
    --region $REGION \
    --group-name talos-aws-tutorial-sg \
    --description "Security Group for EC2 instances to allow ports required by Talos"

SECURITY_GROUP="(security group id that is returned)"
```

Using the security group from above, allow all internal traffic within the same security group:

```bash
aws ec2 authorize-security-group-ingress \
    --region $REGION \
    --group-name talos-aws-tutorial-sg \
    --protocol all \
    --port 0 \
    --source-group talos-aws-tutorial-sg
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

```bash
LOAD_BALANCER_ARN="(arn of the load balancer)"
```

```bash
aws elbv2 create-target-group \
    --region $REGION \
    --name talos-aws-tutorial-tg \
    --protocol TCP \
    --port 6443 \
    --target-type ip \
    --vpc-id $VPC
```

Also note the `TargetGroupArn` that is returned.

```bash
TARGET_GROUP_ARN="(target group arn)"
```

### Create the Machine Configuration Files

Using the DNS name of the loadbalancer created earlier, generate the base configuration files for the Talos machines.
> Note that the `port` used here is the externally accessible port configured on the load balancer - 443 - not the internal port of 6443:

```bash
$ talosctl gen config talos-k8s-aws-tutorial https://<load balancer DNS>:<port> --with-examples=false --with-docs=false
created controlplane.yaml
created worker.yaml
created talosconfig
```

> Note that the generated configs are too long for AWS userdata field if the `--with-examples` and `--with-docs` flags are not passed.

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

> change the instance type if desired.
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

> Make a note of the resulting `PrivateIpAddress` from the controlplane nodes for later use.

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

Now, using the load balancer target group's ARN, and the **PrivateIpAddress** from the controlplane instances that you created :

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

Set the `endpoints` (the control plane node to which `talosctl` commands are sent) and `nodes` (the nodes that the command operates on):

```bash
talosctl --talosconfig talosconfig config endpoint <control plane 1 PUBLIC IP>
talosctl --talosconfig talosconfig config node <control plane 1 PUBLIC IP>
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

The different control plane nodes should send/receive traffic via the load balancer, notice that one of the control plane has intiated the etcd cluster, and the others should join.
You can now watch as your cluster bootstraps, by using

```bash
talosctl --talosconfig talosconfig  health
```

You can also watch the performance of a node, via:

```bash
talosctl  --talosconfig talosconfig dashboard
```

And use standard `kubectl` commands.
