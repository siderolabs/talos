#!/bin/bash

set -eo pipefail

cd pki

IP_ADDR="${1}"
CERT_LENGTH=$(( 24 * 365 * 1 ))
NODE="master-1"

if [[ -z ${OSCTL} ]]; then
	if [[ $(uname -s) == "Linux" ]]; then
		OSCTL="../../../build/osctl-linux-amd64"
	elif [[ $(uname -s) == "Darwin" ]]; then
		OSCTL="../../../build/osctl-darwin-amd64"
	fi
fi

# OS PKI

echo "Generating OS PKI"
${OSCTL} gen ca --hours ${CERT_LENGTH} --organization talos

# Kubernetes PKI

echo "Generating Kubernetes PKI"
${OSCTL} gen ca --rsa --hours ${CERT_LENGTH} --organization kubernetes

# User PKI

echo "Generating user PKI"
${OSCTL} gen key --name developer
${OSCTL} gen csr --ip 127.0.0.1 --key developer.key
${OSCTL} gen crt \
	--hours ${CERT_LENGTH} \
	--ca talos \
	--csr developer.csr \
	--name developer


echo "Injecting PKI ( kubernetes/OS )"
for node in master-1 master-2 master-3; do
  cp ../userdata/.${node}.tpl.yaml ../userdata/${node}.yaml
  chmod 600 ../userdata/${node}.yaml

  # Inject OS PKI
  ${OSCTL} inject os \
  	--crt talos.crt \
  	--key talos.key \
  	../userdata/${node}.yaml

  # Inject Kubernetes PKI
  ${OSCTL} inject kubernetes \
  	--crt kubernetes.crt \
  	--key kubernetes.key \
  	../userdata/${node}.yaml
done

# Inject bootstrap/init token for master-1
echo "Injecting init token"
${OSCTL} inject token ../userdata/master-1.yaml

cp ../userdata/.worker.tpl.yaml ../userdata/worker-1.yaml

# Configure osctl

touch ../talosconfig
${OSCTL} config add "talos-local" \
	--ca talos.crt \
	--crt developer.crt \
	--key developer.key
${OSCTL} config context "talos-local"
${OSCTL} config target "${IP_ADDR}"

