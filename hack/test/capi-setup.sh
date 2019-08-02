#!/bin/bash
set -eou pipefail

source ./hack/test/e2e-runner.sh

## Create tmp dir
mkdir -p $TMP

## Drop in capi stuff
sed -i "s/{{PACKET_AUTH_TOKEN}}/${PACKET_AUTH_TOKEN}/" ${PWD}/hack/test/manifests/provider-components.yaml
sed -i "s#{{GCE_SVC_ACCT}}#${GCE_SVC_ACCT}#" ${PWD}/hack/test/manifests/capi-secrets.yaml
sed -i "s#{{AZURE_SVC_ACCT}}#${AZURE_SVC_ACCT}#" ${PWD}/hack/test/manifests/capi-secrets.yaml
e2e_run "kubectl apply -f /e2emanifests/provider-components.yaml -f /e2emanifests/capi-secrets.yaml"

## Wait for talosconfig in cm then dump it out
e2e_run "timeout=\$((\$(date +%s) + ${TIMEOUT}))
		 until kubectl wait --timeout=1s --for=condition=Ready -n cluster-api-provider-talos-system pod/cluster-api-provider-talos-controller-manager-0
		 do
		   if  [[ \$(date +%s) -gt \$timeout ]]
		   then
		     exit 1
		   fi
		   echo 'Waiting to CAPT pod to be available...'
		   sleep 10
		 done"