---
title: Pull Through Image Cache
description: "How to set up local transparent container images caches."
aliases:
  - ../../guides/configuring-pull-through-cache
---

In this guide we will create a set of local caching Docker registry proxies to minimize local cluster startup time.

When running Talos locally, pulling images from container registries might take a significant amount of time.
We spin up local caching pass-through registries to cache images and configure a local Talos cluster to use those proxies.
A similar approach might be used to run Talos in production in air-gapped environments.
It can be also used to verify that all the images are available in local registries.

## Video Walkthrough

To see a live demo of this writeup, see the video below:

<iframe width="560" height="315" src="https://www.youtube.com/embed/PRiQJR9Q33s" frameborder="0" allow="accelerometer; autoplay; clipboard-write; encrypted-media; gyroscope; picture-in-picture" allowfullscreen></iframe>

## Requirements

The follow are requirements for creating the set of caching proxies:

- Docker 18.03 or greater
- Local cluster requirements for either [docker]({{< relref "../install/local-platforms/docker" >}}) or [QEMU]({{< relref "../install/local-platforms/qemu" >}}).

## Launch the Caching Docker Registry Proxies

Talos pulls from `docker.io`, `registry.k8s.io`, `gcr.io`, and `ghcr.io` by default.
If your configuration is different, you might need to modify the commands below:

```bash
docker run -d -p 5000:5000 \
    -e REGISTRY_PROXY_REMOTEURL=https://registry-1.docker.io \
    --restart always \
    --name registry-docker.io registry:2

docker run -d -p 5001:5000 \
    -e REGISTRY_PROXY_REMOTEURL=https://registry.k8s.io \
    --restart always \
    --name registry-registry.k8s.io registry:2

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

As a registry container can only handle a single upstream Docker registry, we launch a container per upstream, each on its own
host port (5000, 5001, 5002, 5003 and 5004).

## Using Caching Registries with `QEMU` Local Cluster

With a [QEMU]({{< relref "../install/local-platforms/qemu" >}}) local cluster, a bridge interface is created on the host.
As registry containers expose their ports on the host, we can use bridge IP to direct proxy requests.

```bash
sudo talosctl cluster create --provisioner qemu \
    --registry-mirror docker.io=http://10.5.0.1:5000 \
    --registry-mirror registry.k8s.io=http://10.5.0.1:5001 \
    --registry-mirror gcr.io=http://10.5.0.1:5003 \
    --registry-mirror ghcr.io=http://10.5.0.1:5004
```

The Talos local cluster should now start pulling via caching registries.
This can be verified via registry logs, e.g. `docker logs -f registry-docker.io`.
The first time cluster boots, images are pulled and cached, so next cluster boot should be much faster.

> Note: `10.5.0.1` is a bridge IP with default network (`10.5.0.0/24`), if using custom `--cidr`, value should be adjusted accordingly.

## Using Caching Registries with `docker` Local Cluster

With a [docker]({{< relref "../install/local-platforms/docker" >}}) local cluster we can use docker bridge IP, default value for that IP is `172.17.0.1`.
On Linux, the docker bridge address can be inspected with `ip addr show docker0`.

```bash
talosctl cluster create --provisioner docker \
    --registry-mirror docker.io=http://172.17.0.1:5000 \
    --registry-mirror registry.k8s.io=http://172.17.0.1:5001 \
    --registry-mirror gcr.io=http://172.17.0.1:5003 \
    --registry-mirror ghcr.io=http://172.17.0.1:5004
```

## Machine Configuration

The caching registries can be configured via machine configuration [patch]({{< relref "patching" >}}), equivalent to the command line flags above:

```yaml
machine:
  registries:
    mirrors:
      docker.io:
        endpoints:
          - http://10.5.0.1:5000
      gcr.io:
        endpoints:
          - http://10.5.0.1:5003
      ghcr.io:
        endpoints:
          - http://10.5.0.1:5004
      registry.k8s.io:
        endpoints:
          - http://10.5.0.1:5001
```

## Cleaning Up

To cleanup, run:

```bash
docker rm -f registry-docker.io
docker rm -f registry-registry.k8s.io
docker rm -f registry-gcr.io
docker rm -f registry-ghcr.io
```

> Note: Removing docker registry containers also removes the image cache.
> So if you plan to use caching registries, keep the containers running.

## Using Harbor as a Caching Registry

[Harbor](https://goharbor.io/) is an open source container registry that can be used as a caching proxy.
Harbor supports configuring multiple upstream registries, so it can be used to cache multiple registries at once behind a single endpoint.

![Harbor Endpoints](/images/harbor-endpoints.png)

![Harbor Projects](/images/harbor-projects.png)

As Harbor puts a registry name in the pull image path, we need to set `overridePath: true` to prevent Talos and containerd from appending `/v2` to the path.

```yaml
machine:
  registries:
    mirrors:
      docker.io:
        endpoints:
          - http://harbor/v2/proxy-docker.io
        overridePath: true
      ghcr.io:
        endpoints:
          - http://harbor/v2/proxy-ghcr.io
        overridePath: true
      gcr.io:
        endpoints:
          - http://harbor/v2/proxy-gcr.io
        overridePath: true
      registry.k8s.io:
        endpoints:
          - http://harbor/v2/proxy-registry.k8s.io
        overridePath: true
```

The Harbor external endpoint (`http://harbor` in this example) can be configured with authentication or custom TLS:

```yaml
machine:
  registries:
    config:
      harbor:
        auth:
          username: admin
          password: password
```
