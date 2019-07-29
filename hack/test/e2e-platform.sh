source ./hack/test/e2e-runner.sh

## Cleanup the platform resources upon any exit
cleanup() {
 e2e_run "kubectl delete machine talos-e2e-${PLATFORM}-master-0 talos-e2e-${PLATFORM}-master-1 talos-e2e-${PLATFORM}-master-2
          kubectl scale machinedeployment talos-e2e-${PLATFORM}-workers --replicas=0
          kubectl delete machinedeployment talos-e2e-${PLATFORM}-workers
          kubectl delete cluster talos-e2e-${PLATFORM}"
}

trap cleanup EXIT

## Download kustomize and template out capi cluster, then deploy it
e2e_run "kubectl apply -f /e2emanifests/${PLATFORM}-cluster.yaml"		   

## Wait for talosconfig in cm then dump it out
e2e_run "timeout=\$((\$(date +%s) + ${TIMEOUT}))
		 until kubectl get cm -n cluster-api-provider-talos-system talos-e2e-${PLATFORM}-master-0
		 do
		   if  [[ \$(date +%s) -gt \$timeout ]]
		   then
		     exit 1
		   fi
		   sleep 10
		 done
         kubectl get cm -n cluster-api-provider-talos-system talos-e2e-${PLATFORM}-master-0 -o jsonpath='{.data.talosconfig}' > ${TALOSCONFIG}-${PLATFORM}-capi"

## Wait for kubeconfig from capi master-0
e2e_run "timeout=\$((\$(date +%s) + ${TIMEOUT}))
		 until /bin/osctl --talosconfig ${TALOSCONFIG}-${PLATFORM}-capi kubeconfig > ${KUBECONFIG}-${PLATFORM}-capi
		 do
		   if  [[ \$(date +%s) -gt \$timeout ]]
		   then
		     exit 1
		   fi
		   sleep 10
		 done"

##  Wait for nodes to check in
e2e_run "timeout=\$((\$(date +%s) + ${TIMEOUT}))
		 until KUBECONFIG=${KUBECONFIG}-${PLATFORM}-capi kubectl get nodes -o json | jq '.items | length' | grep ${NUM_NODES} >/dev/null
		 do
		   if  [[ \$(date +%s) -gt \$timeout ]]
		   then
			exit 1
		   fi
		   KUBECONFIG=${KUBECONFIG}-${PLATFORM}-capi kubectl get nodes -o wide
		   sleep 10
		 done"

##  Apply psp and flannel
e2e_run "KUBECONFIG=${KUBECONFIG}-${PLATFORM}-capi kubectl apply -f /manifests/psp.yaml -f /manifests/flannel.yaml"

## Wait for kube-proxy up
e2e_run "timeout=\$((\$(date +%s) + ${TIMEOUT}))
		 until KUBECONFIG=${KUBECONFIG}-${PLATFORM}-capi kubectl get po -n kube-system -l k8s-app=kube-proxy -o json | jq '.items | length' | grep ${NUM_NODES} > /dev/null
		 do
		   if  [[ \$(date +%s) -gt \$timeout ]]
		   then
			exit 1
		   fi
		   KUBECONFIG=${KUBECONFIG}-${PLATFORM}-capi kubectl get po -n kube-system -l k8s-app=kube-proxy
		   sleep 10
		 done"

##  Wait for nodes ready
e2e_run "KUBECONFIG=${KUBECONFIG}-${PLATFORM}-capi kubectl wait --timeout=${TIMEOUT}s --for=condition=ready=true --all nodes"

## Verify that we have an HA controlplane
e2e_run "timeout=\$((\$(date +%s) + ${TIMEOUT}))
		 until KUBECONFIG=${KUBECONFIG}-${PLATFORM}-capi kubectl get nodes -l node-role.kubernetes.io/master='' -o json | jq '.items | length' | grep 3 > /dev/null
		 do
		   if  [[ \$(date +%s) -gt \$timeout ]]
		   then
			exit 1
		   fi
		   KUBECONFIG=${KUBECONFIG}-${PLATFORM}-capi kubectl get nodes -l node-role.kubernetes.io/master='' -o json | jq '.items | length'
		   sleep 10
		 done"

## Download sonobuoy and run conformance
e2e_run "apt-get update && apt-get install wget
		 wget --quiet -O /tmp/sonobuoy.tar.gz ${SONOBUOY_URL}
		 tar -xf /tmp/sonobuoy.tar.gz -C /usr/local/bin
		 sonobuoy run --kubeconfig ${KUBECONFIG}-${PLATFORM}-capi --wait --skip-preflight --plugin e2e
		 results=\$(sonobuoy retrieve --kubeconfig ${KUBECONFIG}-${PLATFORM}-capi)
		 sonobuoy e2e --kubeconfig ${KUBECONFIG}-${PLATFORM}-capi \$results"
