---
title: 'Running Behind a Corporate Proxy'
---

## Creating an Image for MITM Proxies

Create a `Dockerfile` that will be used to customize the installer image:

```dockerfile
FROM alpine AS ca
COPY --from=docker.io/autonomy/ca-certificates:febbf49 / /rootfs
COPY ca.crt /tmp/ca.crt
RUN cat /tmp/ca.crt >> /rootfs/etc/ssl/certs/ca-certificates.crt

FROM scratch AS customization
COPY --from=ca /rootfs /

FROM docker.io/autonomy/installer:latest
COPY --from=customization / /
```

Build the image:

```bash
docker build -t <organization>/installer:latest .
```

> Note: You can use the `--squash` flag to create smaller images.

At this point, you will need to generate an image for your desired platform.
We will use VMware as an example:

```bash
docker run --rm -v /dev:/dev -v $PWD/build:/out \
    --privileged \
    <organization>/installer:latest \
    install \
    -r \
    -p vmware \
    -u guestinfo \
    -e console=tty0 \
    -e earlyprintk=ttyS0,115200
docker run --rm -v /dev:/dev -v $PWD/build:/out \
    --privileged \
    <organization>/installer:latest \
    ova
```

## Configuring a Machine to Use the Proxy

To make use of a proxy:

```yaml
machine:
  env:
    http_proxy: <http proxy>
    https_proxy: <https proxy>
    no_proxy: <no proxy>
```

Additionally, configure the DNS `nameservers`, and NTP `servers`:

```yaml
machine:
  env:
  ...
  time:
    servers:
      - <server 1>
      - <server ...>
      - <server n>
  ...
  network:
    nameservers:
      - <ip 1>
      - <ip ...>
      - <ip n>
```
