---
title: "AWS"
description: "Creating a cluster via the AWS CLI."
aliases:
  - ../../../cloud-platforms/aws
---

## Creating a Cluster via the AWS CLI

In this guide we will create an HA Kubernetes cluster with 3 control plane nodes across 3 availability zones.
You should have an existing AWS account and have the AWS CLI installed and configured.
If you need more information on AWS specifics, please see the [official AWS documentation](https://docs.aws.amazon.com).

To install the dependencies for this tutorial you can use homebrew on macOS or Linux:

```bash
brew install siderolabs/tap/talosctl kubectl jq curl xz
```

If you would like to create infrastructure via `terraform` or `opentofu` please see the example in the [contrib repository](https://github.com/siderolabs/contrib/tree/main/examples/terraform/aws).

> Note: this guide is not a production set up and steps were tested in `bash` and `zsh` shells.

### Create AWS Resources

We will be creating a control plane with 3 Ec2 instances spread across 3 availability zones.
It is recommended to not use the default VPC so we will create a new one for this tutorial.

Change to your desired region and CIDR block and create a VPC:

> Make sure your subnet does not overlap with `10.244.0.0/16` or `10.96.0.0/12` the [default pod and services subnets in Kubernetes]({{% relref "../../../introduction/troubleshooting.md#conflict-on-kubernetes-and-host-subnets" %}}).

```bash
AWS_REGION="us-west-2"
IPV4_CIDR="10.1.0.0/18"
VPC_ID=$(aws ec2 create-vpc \
    --cidr-block $IPV4_CIDR \
    --output text --query 'Vpc.VpcId')
```

### Create the Subnets

Create 3 smaller CIDRs to use for each subnet in different availability zones.
Make sure to adjust these CIDRs if you changed the default value from the last command.

```bash
IPV4_CIDRS=( "10.1.0.0/22" "10.1.4.0/22" "10.1.8.0/22" )
```

Next create a subnet in each availability zones.

> Note: If you're using zsh you need to run `setopt KSH_ARRAYS` to have arrays referenced properly.

```bash
CIDR=0
declare -a SUBNETS
AZS=($(aws ec2 describe-availability-zones \
    --query 'AvailabilityZones[].ZoneName' \
    --filter "Name=state,Values=available" \
    --output text | tr -s '\t' '\n' | head -n3))

for AZ in ${AZS[@]}; do
        SUBNETS[$CIDR]=$(aws ec2 create-subnet \
            --vpc-id $VPC_ID \
            --availability-zone $AZ \
            --cidr-block ${IPV4_CIDRS[$CIDR]} \
            --query 'Subnet.SubnetId' \
            --output text)
        aws ec2 modify-subnet-attribute \
            --subnet-id ${SUBNETS[$CIDR]} \
            --private-dns-hostname-type-on-launch resource-name
        echo ${SUBNETS[$CIDR]}
        ((CIDR++))
done
```

Create an internet gateway and attach it to the VPC:

```bash
IGW_ID=$(aws ec2 create-internet-gateway \
    --query 'InternetGateway.InternetGatewayId' \
    --output text)

aws ec2 attach-internet-gateway \
    --vpc-id $VPC_ID \
    --internet-gateway-id $IGW_ID

ROUTE_TABLE_ID=$(aws ec2 describe-route-tables \
        --filters "Name=vpc-id,Values=$VPC_ID" \
        --query 'RouteTables[].RouteTableId' \
        --output text)

aws ec2 create-route \
    --route-table-id $ROUTE_TABLE_ID \
    --destination-cidr-block 0.0.0.0/0 \
    --gateway-id $IGW_ID
```

### Official AMI Images

Official AMI image ID can be found in the `cloud-images.json` file attached to the [Talos release](https://github.com/siderolabs/talos/releases).

```bash
AMI=$(curl -sL https://github.com/siderolabs/talos/releases/download/{{< release >}}/cloud-images.json | \
    jq -r '.[] | select(.region == "'$AWS_REGION'") | select (.arch == "amd64") | .id')
echo $AMI
```

If using the official AMIs, you can skip to [Creating the Security group]({{< relref "#create-a-security-group" >}})

### Create your own AMIs

> The use of the official Talos AMIs are recommended, but if you wish to build your own AMIs, follow the procedure below.

#### Create the S3 Bucket

```bash
aws s3api create-bucket \
    --bucket $BUCKET \
    --create-bucket-configuration LocationConstraint=$AWS_REGION \
    --acl private
```

#### Create the `vmimport` Role

In order to create an AMI, ensure that the `vmimport` role exists as described in the [official AWS documentation](https://docs.aws.amazon.com/vm-import/latest/userguide/required-permissions.html).

Note that the role should be associated with the S3 bucket we created above.

#### Create the Image Snapshot

First, download the AWS image from Image Factory:

```bash
curl -L https://factory.talos.dev/image/376567988ad370138ad8b2698212367b8edcb69b5fd68c80be1f2ec7d603b4ba/{{< release >}}/aws-amd64.raw.xz | xz -d > disk.raw
```

Copy the RAW disk to S3 and import it as a snapshot:

```bash
aws s3 cp disk.raw s3://$BUCKET/talos-aws-tutorial.raw
$SNAPSHOT_ID=$(aws ec2 import-snapshot \
    --region $REGION \
    --description "Talos kubernetes tutorial" \
    --disk-container "Format=raw,UserBucket={S3Bucket=$BUCKET,S3Key=talos-aws-tutorial.raw}" \
    --query 'SnapshotId' \
    --output text)
```

To check on the status of the import, run:

```bash
aws ec2 describe-import-snapshot-tasks \
    --import-task-ids
```

Once the `SnapshotTaskDetail.Status` indicates `completed`, we can register the image.

#### Register the Image

```bash
AMI=$(aws ec2 register-image \
    --block-device-mappings "DeviceName=/dev/xvda,VirtualName=talos,Ebs={DeleteOnTermination=true,SnapshotId=$SNAPSHOT_ID,VolumeSize=4,VolumeType=gp2}" \
    --root-device-name /dev/xvda \
    --virtualization-type hvm \
    --architecture x86_64 \
    --ena-support \
    --name talos-aws-tutorial-ami \
    --query 'ImageId' \
    --output text)
```

We now have an AMI we can use to create our cluster.

### Create a Security Group

```bash
SECURITY_GROUP_ID=$(aws ec2 create-security-group \
    --vpc-id $VPC_ID \
    --group-name talos-aws-tutorial-sg \
    --description "Security Group for EC2 instances to allow ports required by Talos" \
    --query 'GroupId' \
    --output text)
```

Using the security group from above, allow all internal traffic within the same security group:

```bash
aws ec2 authorize-security-group-ingress \
    --group-id $SECURITY_GROUP_ID \
    --protocol all \
    --port 0 \
    --source-group $SECURITY_GROUP_ID
```

Expose the Talos (50000) and Kubernetes API.

> Note: This is only required for the control plane nodes.
> For a production environment you would want separate private subnets for worker nodes.

```bash
aws ec2 authorize-security-group-ingress \
    --group-id $SECURITY_GROUP_ID \
    --ip-permissions \
        IpProtocol=tcp,FromPort=50000,ToPort=50000,IpRanges="[{CidrIp=0.0.0.0/0}]" \
        IpProtocol=tcp,FromPort=6443,ToPort=6443,IpRanges="[{CidrIp=0.0.0.0/0}]" \
    --query 'SecurityGroupRules[].SecurityGroupRuleId' \
    --output text
```

We will bootstrap Talos with a MachineConfig via user-data it will never be exposed to the internet without certificate authentication.

We enable KubeSpan in this tutorial so you need to allow inbound UDP for the Wireguard port:

```bash
aws ec2 authorize-security-group-ingress \
    --group-id $SECURITY_GROUP_ID \
    --ip-permissions \
        IpProtocol=tcp,FromPort=51820,ToPort=51820,IpRanges="[{CidrIp=0.0.0.0/0}]" \
    --query 'SecurityGroupRules[].SecurityGroupRuleId' \
    --output text
```

### Create a Load Balancer

The load balancer is used for a stable Kubernetes API endpoint.

```bash
LOAD_BALANCER_ARN=$(aws elbv2 create-load-balancer \
    --name talos-aws-tutorial-lb \
    --subnets $(echo ${SUBNETS[@]}) \
    --type network \
    --ip-address-type ipv4 \
    --query 'LoadBalancers[].LoadBalancerArn' \
    --output text)

LOAD_BALANCER_DNS=$(aws elbv2 describe-load-balancers \
    --load-balancer-arns $LOAD_BALANCER_ARN \
    --query 'LoadBalancers[].DNSName' \
    --output text)
```

Now create a target group for the load balancer:

```bash
TARGET_GROUP_ARN=$(aws elbv2 create-target-group \
    --name talos-aws-tutorial-tg \
    --protocol TCP \
    --port 6443 \
    --target-type instance \
    --vpc-id $VPC_ID \
    --query 'TargetGroups[].TargetGroupArn' \
    --output text)

LISTENER_ARN=$(aws elbv2 create-listener \
    --load-balancer-arn $LOAD_BALANCER_ARN \
    --protocol TCP \
    --port 6443 \
    --default-actions Type=forward,TargetGroupArn=$TARGET_GROUP_ARN \
    --query 'Listeners[].ListenerArn' \
    --output text)
```

### Create the Machine Configuration Files

We will create a [machine config patch]({{% relref "../../../talos-guides/configuration/patching.md#rfc6902-json-patches" %}}) to use the AWS time servers.
You can create [additional patches]({{% relref "../../../reference/configuration/v1alpha1/config.md" %}}) to customize the configuration as needed.

```bash
cat <<EOF > time-server-patch.yaml
machine:
  time:
    servers:
      - 169.254.169.123
EOF
```

Using the DNS name of the loadbalancer created earlier, generate the base configuration files for the Talos machines.

```bash
talosctl gen config talos-k8s-aws-tutorial https://${LOAD_BALANCER_DNS}:6443 \
    --with-examples=false \
    --with-docs=false \
    --with-kubespan \
    --install-disk /dev/xvda \
    --config-patch '@time-server-patch.yaml'
```

> Note that the generated configs are too long for AWS userdata field if the `--with-examples` and `--with-docs` flags are not passed.

### Create the EC2 Instances

> Note: There is a known issue that prevents Talos from running on T2 instance types.
> Please use T3 if you need burstable instance types.

#### Create the Control Plane Nodes

```bash
declare -a CP_INSTANCES
INSTANCE_INDEX=0
for SUBNET in ${SUBNETS[@]}; do
    CP_INSTANCES[${INSTANCE_INDEX}]=$(aws ec2 run-instances \
        --image-id $AMI \
        --subnet-id $SUBNET \
        --instance-type t3.small \
        --user-data file://controlplane.yaml \
        --associate-public-ip-address \
        --security-group-ids $SECURITY_GROUP_ID \
        --count 1 \
        --tag-specifications "ResourceType=instance,Tags=[{Key=Name,Value=talos-aws-tutorial-cp-$INSTANCE_INDEX}]" \
        --query 'Instances[].InstanceId' \
        --output text)
    echo ${CP_INSTANCES[${INSTANCE_INDEX}]}
    ((INSTANCE_INDEX++))
done
```

#### Create the Worker Nodes

For the worker nodes we will create a new launch template with the `worker.yaml` machine configuration and create an autoscaling group.

```bash
WORKER_LAUNCH_TEMPLATE_ID=$(aws ec2 create-launch-template \
    --launch-template-name talos-aws-tutorial-worker \
    --launch-template-data '{
        "ImageId":"'$AMI'",
        "InstanceType":"t3.small",
        "UserData":"'$(base64 -w0 worker.yaml)'",
        "NetworkInterfaces":[{
            "DeviceIndex":0,
            "AssociatePublicIpAddress":true,
            "Groups":["'$SECURITY_GROUP_ID'"],
            "DeleteOnTermination":true
        }],
        "BlockDeviceMappings":[{
            "DeviceName":"/dev/xvda",
            "Ebs":{
                "VolumeSize":20,
                "VolumeType":"gp3",
                "DeleteOnTermination":true
            }
        }],
        "TagSpecifications":[{
            "ResourceType":"instance",
            "Tags":[{
          "Key":"Name",
          "Value":"talos-aws-tutorial-worker"
          }]
        }]}' \
    --query 'LaunchTemplate.LaunchTemplateId' \
    --output text)
```

```bash
aws autoscaling create-auto-scaling-group \
    --auto-scaling-group-name talos-aws-tutorial-worker \
    --min-size 1 \
    --max-size 3 \
    --desired-capacity 1 \
    --availability-zones $(echo ${AZS[@]}) \
    --target-group-arns $TARGET_GROUP_ARN \
    --launch-template "LaunchTemplateId=${WORKER_LAUNCH_TEMPLATE_ID}" \
    --vpc-zone-identifier $(echo ${SUBNETS[@]} | tr ' ' ',')
```

### Configure the Load Balancer

Now, using the load balancer target group's ARN, and the **PrivateIpAddress** from the controlplane instances that you created :

```bash
for INSTANCE in ${CP_INSTANCES[@]}; do
    aws elbv2 register-targets \
    --target-group-arn $TARGET_GROUP_ARN \
    --targets Id=$(aws ec2 describe-instances \
        --instance-ids $INSTANCE \
        --query 'Reservations[].Instances[].InstanceId' \
        --output text)
done
```

### Export the `talosconfig` file

Export the `talosconfig` file so commands sent to Talos will be authenticated.

```bash
export TALOSCONFIG=$(pwd)/talosconfig
```

### Bootstrap `etcd`

```bash
WORKER_INSTANCES=( $(aws autoscaling \
    describe-auto-scaling-instances \
    --query 'AutoScalingInstances[?AutoScalingGroupName==`talos-aws-tutorial-worker`].InstanceId' \
    --output text) )
```

Set the `endpoints` (the control plane node to which `talosctl` commands are sent) and `nodes` (the nodes that the command operates on):

```bash
talosctl config endpoints $(aws ec2 describe-instances \
    --instance-ids ${CP_INSTANCES[*]} \
    --query 'Reservations[].Instances[].PublicIpAddress' \
    --output text)

talosctl config nodes $(aws ec2 describe-instances \
    --instance-ids $(echo ${CP_INSTANCES[1]}) \
    --query 'Reservations[].Instances[].PublicIpAddress' \
    --output text)
```

Bootstrap `etcd`:

```bash
talosctl bootstrap
```

You can now watch as your cluster bootstraps, by using

```bash
talosctl health
```

This command will take a few minutes for the nodes to start etcd, reach quorum and start the Kubernetes control plane.

You can also watch the performance of a node, via:

```bash
talosctl dashboard
```

### Retrieve the `kubeconfig`

When the cluster is healthy you can retrieve the admin `kubeconfig` by running:

```bash
talosctl kubeconfig .
export KUBECONFIG=$(pwd)/kubeconfig
```

And use standard `kubectl` commands.

```bash
kubectl get nodes
```

## Cleanup resources

If you would like to delete all of the resources you created during this tutorial you can run the following commands.

```bash
aws elbv2 delete-listener --listener-arn $LISTENER_ARN
aws elbv2 delete-target-group --target-group-arn $TARGET_GROUP_ARN
aws elbv2 delete-load-balancer --load-balancer-arn $LOAD_BALANCER_ARN

aws autoscaling update-auto-scaling-group \
    --auto-scaling-group-name talos-aws-tutorial-worker \
    --min-size 0 \
    --max-size 0 \
    --desired-capacity 0

aws ec2 terminate-instances --instance-ids ${CP_INSTANCES[@]} ${WORKER_INSTANCES[@]} \
    --query 'TerminatingInstances[].InstanceId' \
    --output text

aws autoscaling delete-auto-scaling-group \
    --auto-scaling-group-name talos-aws-tutorial-worker \
    --force-delete

aws ec2 delete-launch-template --launch-template-id $WORKER_LAUNCH_TEMPLATE_ID

while $(aws ec2 describe-instances \
    --instance-ids ${CP_INSTANCES[@]} ${WORKER_INSTANCES[@]} \
    --query 'Reservations[].Instances[].[InstanceId,State.Name]' \
    --output text | grep -q shutting-down); do \
        echo "waiting for instances to terminate"; sleep 5s
done

aws ec2 detach-internet-gateway --vpc-id $VPC_ID --internet-gateway-id $IGW_ID
aws ec2 delete-internet-gateway --internet-gateway-id $IGW_ID

aws ec2 delete-security-group --group-id $SECURITY_GROUP_ID

for SUBNET in ${SUBNETS[@]}; do
    aws ec2 delete-subnet --subnet-id $SUBNET
done

aws ec2 delete-vpc --vpc-id $VPC_ID

rm -f controlplane.yaml worker.yaml talosconfig kubeconfig time-server-patch.yaml disk.raw
```
