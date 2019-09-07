#!/bin/bash

set -eou pipefail

REGION="us-east-1"
BUCKET="talos-ci-e2e"
TMP=/tmp/e2e/aws

## Setup svc account
mkdir -p ${TMP}
echo ${AWS_SVC_ACCT} | base64 -d > ${TMP}/svc-acct.ini

# Ensure AWS_ACCESS_KEY_ID and AWS_SECRET_ACCESS_KEY env vars are available
export AWS_ACCESS_KEY_ID=$(awk '/aws_access_key_id/ { print $NF }' ${TMP}/svc-acct.ini)
export AWS_SECRET_ACCESS_KEY=$(awk '/aws_secret_access_key/ { print $NF }' ${TMP}/svc-acct.ini)

# Ensure bucket exists ( already done )
#aws s3api create-bucket --region ${REGION} --bucket ${BUCKET} --acl private

## Untar image
tar -C ${TMP} -xf ./build/aws.tar.gz

# Upload Image
echo "uploading image to s3"
aws s3 cp --quiet ${TMP}/aws.raw s3://${BUCKET}/aws-${TAG}.raw

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
ami=$(aws ec2 register-image --region ${REGION} \
                       --block-device-mappings "DeviceName=/dev/xvda,VirtualName=talostest,Ebs={DeleteOnTermination=true,SnapshotId=${snapshot_id},VolumeSize=20,VolumeType=gp2}" \
                       --root-device-name /dev/xvda \
                       --virtualization-type hvm \
                       --architecture x86_64 \
                       --ena-support \
                       --name talos-e2e-${TAG} | \
      jq -r '.ImageId')

## Setup the cluster YAML.
sed -e "s#{{REGION}}#${REGION}#g" \
    -e "s/{{TAG}}/${TAG}/" \
    -e "s#{{AMI}}#${ami}#g" ${PWD}/hack/test/manifests/aws-cluster.yaml > ${TMP}/cluster.yaml
