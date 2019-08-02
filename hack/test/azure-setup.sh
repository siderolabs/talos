#!/bin/bash

set -eou pipefail

STORAGE_ACCOUNT=talostesting
STORAGE_CONTAINER=talostesting
GROUP=talos
TMP=/tmp/e2e

azcli_run() {
	docker run \
	 	--rm \
		--interactive \
		--entrypoint=bash \
		--mount type=bind,source=${TMP},target=${TMP} \
	 	mcr.microsoft.com/azure-cli -c "az login --service-principal --username ${CLIENT_ID} \
		                                --password ${CLIENT_SECRET} --tenant ${TENANT_ID} > /dev/null && \
										${1}"
}

## Setup svc acct vars
mkdir -p ${TMP}
echo ${AZURE_SVC_ACCT} | base64 -d > ${TMP}/svc-acct.json
CLIENT_ID="$( cat ${TMP}/svc-acct.json | jq -r '.clientId' )"
CLIENT_SECRET="$( cat ${TMP}/svc-acct.json | jq -r '.clientSecret' )"
TENANT_ID="$( cat ${TMP}/svc-acct.json | jq -r '.tenantId' )"

## Untar image
tar -C ${TMP} -xf ./build/talos-azure.tar.gz

## Login to azure, push blob, create image from blob
AZURE_STORAGE_CONNECTION_STRING=$( azcli_run "az storage account show-connection-string -n ${STORAGE_ACCOUNT} -g ${GROUP} -o tsv" )
           
azcli_run "AZURE_STORAGE_CONNECTION_STRING='${AZURE_STORAGE_CONNECTION_STRING}' az storage blob upload --container-name ${STORAGE_CONTAINER} -f ${TMP}/talos-azure.vhd -n talos-azure.vhd"

azcli_run "az image delete --name talos-e2e -g ${GROUP}"

azcli_run "az image create --name talos-e2e --source https://${STORAGE_ACCOUNT}.blob.core.windows.net/${STORAGE_CONTAINER}/talos-azure.vhd --os-type linux -g ${GROUP}"