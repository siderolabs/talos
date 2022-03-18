---
title: Air-gapped Environments
---

In this guide we will create a Talos cluster running in an air-gapped environment with all the required images being pulled from an internal registry.
We will use the [QEMU](../../local-platforms/qemu/) provisioner available in `talosctl` to create a local cluster, but the same approach could be used to deploy Talos in bigger air-gapped networks.

## Requirements

The follow are requirements for this guide:

- Docker 18.03 or greater
- Requirements for the Talos [QEMU](../../local-platforms/qemu/) cluster

## Identifying Images

In air-gapped environments, access to the public Internet is restricted, so Talos can't pull images from public Docker registries (`docker.io`, `ghcr.io`, etc.)
We need to identify the images required to install and run Talos.
The same strategy can be used for images required by custom workloads running on the cluster.

The `talosctl images` command provides a list of default images used by the Talos cluster (with default configuration
settings).
To print the list of images, run:

```bash
talosctl images
```

This list contains images required by a default deployment of Talos.
There might be additional images required for the workloads running on this cluster, and those should be added to this list.

## Preparing the Internal Registry

As access to the public registries is restricted, we have to run an internal Docker registry.
In this guide, we will launch the registry on the same machine using Docker:

```bash
$ docker run -d -p 6000:5000 --restart always --name registry-aigrapped registry:2
1bf09802bee1476bc463d972c686f90a64640d87dacce1ac8485585de69c91a5
```

This registry will be accepting connections on port 6000 on the host IPs.
The registry is empty by default, so we have fill it with the images required by Talos.

First, we pull all the images to our local Docker daemon:

```bash
$ for image in `talosctl images`; do docker pull $image; done
v0.12.0-amd64: Pulling from coreos/flannel
Digest: sha256:6d451d92c921f14bfb38196aacb6e506d4593c5b3c9d40a8b8a2506010dc3e10
...
```

All images are now stored in the Docker daemon store:

```bash
$ docker images
ghcr.io/talos-systems/install-cni    v0.3.0-12-g90722c3      980d36ee2ee1        5 days ago          79.7MB
k8s.gcr.io/kube-proxy-amd64          v1.20.0                 33c60812eab8        2 weeks ago         118MB
...
```

Now we need to re-tag them so that we can push them to our local registry.
We are going to replace the first component of the image name (before the first slash) with our registry endpoint `127.0.0.1:6000`:

```bash
$ for image in `talosctl images`; do \
    docker tag $image `echo $image | sed -E 's#^[^/]+/#127.0.0.1:6000/#'` \
  done
```

As the next step, we push images to the internal registry:

```bash
$ for image in `talosctl images`; do \
    docker push `echo $image | sed -E 's#^[^/]+/#127.0.0.1:6000/#'` \
  done
```

We can now verify that the images are pushed to the registry:

```bash
$ curl  http://127.0.0.1:6000/v2/_catalog
{"repositories":["autonomy/kubelet","coredns","coreos/flannel","etcd-development/etcd","kube-apiserver-amd64","kube-controller-manager-amd64","kube-proxy-amd64","kube-scheduler-amd64","talos-systems/install-cni","talos-systems/installer"]}
```

> Note: images in the registry don't have the registry endpoint prefix anymore.

## Launching Talos in an Air-gapped Environment

For Talos to use the internal registry, we use the registry mirror feature to redirect all the image pull requests to the internal registry.
This means that the registry endpoint (as the first component of the image reference) gets ignored, and all pull requests are sent directly to the specified endpoint.

We are going to use a QEMU-based Talos cluster for this guide, but the same approach works with Docker-based clusters as well.
As QEMU-based clusters go through the Talos install process, they can be used better to model a real air-gapped environment.

The `talosctl cluster create` command provides conveniences for common configuration options.
The only required flag for this guide is `--registry-mirror '*'=http://10.5.0.1:6000` which redirects every pull request to the internal registry.
The endpoint being used is `10.5.0.1`, as this is the default bridge interface address which will be routable from the QEMU VMs (`127.0.0.1` IP will be pointing to the VM itself).

```bash
$ sudo -E talosctl cluster create --provisioner=qemu --registry-mirror '*'=http://10.5.0.1:6000 --install-image=ghcr.io/talos-systems/installer:v0.15.0
validating CIDR and reserving IPs
generating PKI and tokens
creating state directory in "/home/smira/.talos/clusters/talos-default"
creating network talos-default
creating load balancer
creating dhcpd
creating master nodes
creating worker nodes
waiting for API
...
```

> Note: `--install-image` should match the image which was copied into the internal registry in the previous step.

You can be verify that the cluster is air-gapped by inspecting the registry logs: `docker logs -f registry-airgapped`.

## Closing Notes

Running in an air-gapped environment might require additional configuration changes, for example using custom settings for DNS and NTP servers.

When scaling this guide to the bare-metal environment, following Talos config snippet could be used as an equivalent of the `--registry-mirror` flag above:

```bash
machine:
  ...
  registries:
      mirrors:
      '*':
          endpoints:
          - http://10.5.0.1:6000/
...
```

Other implementations of Docker registry can be used in place of the Docker `registry` image used above to run the registry.
If required, auth can be configured for the internal registry (and custom TLS certificates if needed).
