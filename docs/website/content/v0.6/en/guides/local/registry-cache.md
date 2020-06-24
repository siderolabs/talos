---
title: Registry Cache
---

In this guide we will create a set of local caching Docker registry proxies to minimize local cluster startup time.

When running Talos locally, pulling images from Docker registries might take significant amount of time.
We spin up local  caching pass-through registries to cache images and configure local Talos cluster to use those proxies.
Similar approach might be used to run Talos in production in air-gapped environments.
It can be also used to verify that all the images are available in local registries.

## Requirements

The follow are requirements for creating set of caching proxies:

- Docker 18.03 or greater
- Local cluster requirements for either [docker](docker) or [fireracker](firecracker).

## Launch the Caching Docker Registry Proxies

With default configuration, Talos pulls from `docker.io`, `k8s.gcr.io` and `quay.io`.
If your configuration is different, you might need to modify the commands below:

```bash
docker run -d -p 5000:5000 \
    -e REGISTRY_PROXY_REMOTEURL=https://registry-1.docker.io \
    --restart always \
    --name registry-docker.io registry:2

docker run -d -p 5001:5000 \
    -e REGISTRY_PROXY_REMOTEURL=https://k8s.gcr.io \
    --restart always \
    --name registry-k8s.gcr.io registry:2

docker run -d -p 5002:5000 \
    -e REGISTRY_PROXY_REMOTEURL=https://quay.io \
    --restart always \
    --name registry-quay.io registry:2.5
```

> Note: Proxies are started as docker containers, and they're automatically configured to start with Docker daemon.
Please note that `quay.io` proxy doesn't support recent Docker image schema, so we run older registry image version (2.5).

As a registry container can only handle single upstream Docker registry, we launch a container per upstream, each on its own
host port (5000, 5001, 5002).

## Using Caching Registries with `firecracker` Local Cluster

With [firecracker](firecracker) local cluster, bridge interface is created on the host.
As registry containers expose their ports on the host, we can use bridge IP to direct proxy requests.

```bash
sudo talosctl cluster create --provisioner firecracker \
    --registry-mirror docker.io=http://10.5.0.1:5000 \
    --registry-mirror k8s.gcr.io=http://10.5.0.1:5001 \
    --registry-mirror quay.io=http://10.5.0.1:5002
```

Talos local cluster should now start pulling via caching registries.
This can be verified via registry logs, e.g. `docker logs -f registry-docker.io`.
First time cluster boots, images are pulled and cached, so next cluster boot should be much faster.

> Note: `10.5.0.1` is a bridge IP with default network (`10.5.0.0/24`), if using custom `--cidr`, value should be adjusted accordingly.

## Using Caching Registries with `docker` Local Cluster

With [docker](docker) local clustwer we can use docker bridge IP, default value for that IP is `172.17.0.1`.
On Linux, docker bridge address can be inspected with `ip addr show docker0`.

```bash
talosctl cluster create --provisioner docker \
    --registry-mirror docker.io=http://172.17.0.1:5000 \
    --registry-mirror k8s.gcr.io=http://172.17.0.1:5001 \
    --registry-mirror quay.io=http://172.17.0.1:5002
```

## Cleaning Up

To cleanup, run:

```bash
docker rm -f registry-docker.io
docker rm -f registry-k8s.gcr.io
docker rm -f registry-quay.io
```

> Note: Removing docker registry containers also removes the image cache.
So if you plan to use caching registries, keep containers running.
