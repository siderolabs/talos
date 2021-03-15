#!/bin/bash

set -eou pipefail

source ./hack/test/e2e.sh

REGION="us-east-1"
BUCKET="talos-ci-e2e"

function setup {
  # Setup svc account
  mkdir -p ${TMP}

  # Untar image
  tar -C ${TMP} -xf ${ARTIFACTS}/aws-amd64.tar.gz

  # Upload Image
  echo "uploading image to s3"
  aws s3 cp --quiet ${TMP}/disk.raw s3://${BUCKET}/aws-${TAG}.raw

  # Create snapshot from image
  echo "importing snapshot from s3"
  import_task_id=$(aws ec2 import-snapshot --region ${REGION} --description "talos e2e ${TAG}" --disk-container "Format=raw,UserBucket={S3Bucket=${BUCKET},S3Key=aws-${TAG}.raw}" | jq -r '.ImportTaskId')
  echo ${import_task_id}

  # Wait for import to complete
  echo "waiting for snapshot import to complete"
  snapshot_status=$(aws ec2 describe-import-snapshot-tasks --region ${REGION} --import-task-ids ${import_task_id} | \
                    jq -r --arg image_name "aws-${TAG}.raw" '.ImportSnapshotTasks[] | select(.SnapshotTaskDetail.UserBucket.S3Key == $image_name) | .SnapshotTaskDetail.Status')
  while [ ${snapshot_status} != "completed" ]; do
    sleep 5
    snapshot_status=$(aws ec2 describe-import-snapshot-tasks --region ${REGION} --import-task-ids ${import_task_id} | \
                      jq -r --arg image_name "aws-${TAG}.raw" '.ImportSnapshotTasks[] | select(.SnapshotTaskDetail.UserBucket.S3Key == $image_name) | .SnapshotTaskDetail.Status')
  done
  snapshot_id=$(aws ec2 describe-import-snapshot-tasks --region ${REGION} --import-task-ids ${import_task_id} | \
                jq -r --arg image_name "aws-${TAG}.raw" '.ImportSnapshotTasks[] | select(.SnapshotTaskDetail.UserBucket.S3Key == $image_name) | .SnapshotTaskDetail.SnapshotId')
  echo ${snapshot_id}

  # Create AMI
  image_id=$(aws ec2 describe-images --region ${REGION} --filters="Name=name,Values=talos-e2e-${TAG}" | jq -r '.Images[0].ImageId') || true

  if [[ ${image_id} != "null" ]]; then
    aws ec2 deregister-image --region ${REGION} --image-id ${image_id}
  fi

  ami=$(aws ec2 register-image --region ${REGION} \
          --block-device-mappings "DeviceName=/dev/xvda,VirtualName=talostest,Ebs={DeleteOnTermination=true,SnapshotId=${snapshot_id},VolumeSize=20,VolumeType=gp2}" \
          --root-device-name /dev/xvda \
          --virtualization-type hvm \
          --architecture x86_64 \
          --ena-support \
          --name talos-e2e-${TAG} | jq -r '.ImageId')

  ## Cluster-wide vars
  export CLUSTER_NAME=${NAME_PREFIX}
  export REGION=us-east-1
  export SSH_KEY=talos-e2e
  export VPC_ID=vpc-ff5c5687
  export SUBNET=subnet-c4e9b3a0
  export CLOUD_PROVIDER_VERSION=v1.20.0-alpha.0
  
  ## Control plane vars
  export CP_COUNT=3
  export CP_INSTANCE_TYPE=t3.large
  export CP_VOL_SIZE=50
  export CP_AMI_ID=${ami}
  export CP_ADDL_SEC_GROUPS='[{id: sg-ebe8e59f}]'
  export CP_IAM_PROFILE=CAPI_AWS_ControlPlane

  ## Worker vars
  export WORKER_COUNT=3
  export WORKER_INSTANCE_TYPE=t3.large
  export WORKER_VOL_SIZE=50
  export WORKER_AMI_ID=${ami}
  export WORKER_ADDL_SEC_GROUPS='[{id: sg-ebe8e59f}]'
  export WORKER_IAM_PROFILE=CAPI_AWS_Worker

  ## TODO: update to talos-systems once merged
  ${CLUSTERCTL} config cluster ${NAME_PREFIX} \
    --kubeconfig /tmp/e2e/docker/kubeconfig \
    --from https://github.com/talos-systems/cluster-api-templates/blob/main/aws/standard/standard.yaml > ${TMP}/cluster.yaml
  
}

setup
create_cluster_capi aws
run_talos_integration_test
run_kubernetes_integration_test
