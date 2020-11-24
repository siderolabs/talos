---
title: "Configuring Containerd"
description: ""
---

The base containerd configuration expects to merge in any additional configs present in `/var/cri/conf.d/*.toml`.

## An example of exposing metrics

Into each machine config, add the following:

```yaml
machine:
  ...
  files:
    - content: |
        [metrics]
          address = "0.0.0.0:11234"
      path: /var/cri/conf.d/metrics.toml
      op: create
```

Create cluster like normal and see that metrics are now present on this port:

```bash
$ curl 127.0.0.1:11234/v1/metrics
# HELP container_blkio_io_service_bytes_recursive_bytes The blkio io service bytes recursive
# TYPE container_blkio_io_service_bytes_recursive_bytes gauge
container_blkio_io_service_bytes_recursive_bytes{container_id="0677d73196f5f4be1d408aab1c4125cf9e6c458a4bea39e590ac779709ffbe14",device="/dev/dm-0",major="253",minor="0",namespace="k8s.io",op="Async"} 0
container_blkio_io_service_bytes_recursive_bytes{container_id="0677d73196f5f4be1d408aab1c4125cf9e6c458a4bea39e590ac779709ffbe14",device="/dev/dm-0",major="253",minor="0",namespace="k8s.io",op="Discard"} 0
...
...
```
