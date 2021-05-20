---
title: Configuring Pull Through Cache
---

In this guide we will create a set of local caching Docker registry proxies to minimize local cluster startup time.

When running Talos locally, pulling images from Docker registries might take a significant amount of time.
We spin up local caching pass-through registries to cache images and configure a local Talos cluster to use those proxies.
A similar approach might be used to run Talos in production in air-gapped environments.
It can be also used to verify that all the images are available in local registries.

## Video Walkthrough

To see a live demo of this writeup, see the video below:

<iframe width="560" height="315" src="https://www.youtube.com/embed/PRiQJR9Q33s" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>

## Requirements

The follow are requirements for creating the set of caching proxies:

- Docker 18.03 or greater
- Local cluster requirements for either [docker](../../local-platforms/docker/) or [QEMU](../../local-platforms/qemu/).

## Launch the Caching Docker Registry Proxies

Talos pulls from `docker.io`, `k8s.gcr.io`, `quay.io`, `gcr.io`, and `ghcr.io` by default.
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

docker run -d -p 5003:5000 \
    -e REGISTRY_PROXY_REMOTEURL=https://gcr.io \
    --restart always \
    --name registry-gcr.io registry:2

docker run -d -p 5004:5000 \
    -e REGISTRY_PROXY_REMOTEURL=https://ghcr.io \
    --restart always \
    --name registry-ghcr.io registry:2
```

> Note: Proxies are started as docker containers, and they're automatically configured to start with Docker daemon.
> Please note that `quay.io` proxy doesn't support recent Docker image schema, so we run older registry image version (2.5).

As a registry container can only handle a single upstream Docker registry, we launch a container per upstream, each on its own
host port (5000, 5001, 5002, 5003 and 5004).

## Using Caching Registries with `QEMU` Local Cluster

With a [QEMU](../../local-platforms/qemu/) local cluster, a bridge interface is created on the host.
As registry containers expose their ports on the host, we can use bridge IP to direct proxy requests.

```bash
sudo talosctl cluster create --provisioner qemu \
    --registry-mirror docker.io=http://10.5.0.1:5000 \
    --registry-mirror k8s.gcr.io=http://10.5.0.1:5001 \
    --registry-mirror quay.io=http://10.5.0.1:5002 \
    --registry-mirror gcr.io=http://10.5.0.1:5003 \
    --registry-mirror ghcr.io=http://10.5.0.1:5004
```

The Talos local cluster should now start pulling via caching registries.
This can be verified via registry logs, e.g. `docker logs -f registry-docker.io`.
The first time cluster boots, images are pulled and cached, so next cluster boot should be much faster.

> Note: `10.5.0.1` is a bridge IP with default network (`10.5.0.0/24`), if using custom `--cidr`, value should be adjusted accordingly.

## Using Caching Registries with `docker` Local Cluster

With a [docker](../../local-platforms/docker/) local cluster we can use docker bridge IP, default value for that IP is `172.17.0.1`.
On Linux, the docker bridge address can be inspected with `ip addr show docker0`.

```bash
talosctl cluster create --provisioner docker \
    --registry-mirror docker.io=http://172.17.0.1:5000 \
    --registry-mirror k8s.gcr.io=http://172.17.0.1:5001 \
    --registry-mirror quay.io=http://172.17.0.1:5002 \
    --registry-mirror gcr.io=http://172.17.0.1:5003 \
    --registry-mirror ghcr.io=http://172.17.0.1:5004
```

## Cleaning Up

To cleanup, run:

```bash
docker rm -f registry-docker.io
docker rm -f registry-k8s.gcr.io
docker rm -f registry-quay.io
docker rm -f registry-gcr.io
docker rm -f registry-ghcr.io
```

> Note: Removing docker registry containers also removes the image cache.
> So if you plan to use caching registries, keep the containers running.
