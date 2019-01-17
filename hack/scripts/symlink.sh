#!/bin/bash

PREFIX="${1}"

mkdir -p ${PREFIX}/usr/share
mkdir -p ${PREFIX}/usr/local/share

paths=( /etc/pki /usr/share/ca-certificates /usr/local/share/ca-certificates /etc/ca-certificates )
for d in "${paths[@]}"; do
  ln -sv /etc/ssl/certs ${PREFIX}$d
done

# Required by kube-proxy.
mkdir ${PREFIX}/lib/modules

mkdir -p ${PREFIX}/usr/libexec
mkdir -p ${PREFIX}/var/libexec/kubernetes
ln -sv ../../var/libexec/kubernetes ${PREFIX}/usr/libexec/kubernetes
