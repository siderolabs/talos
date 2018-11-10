---
title: "VMware Fusion"
date: 2018-11-06T06:25:46-08:00
draft: false
weight: 40
menu:
  main:
    parent: 'examples'
    weight: 40
---

```bash
# docker run --rm -it -v /dev:/dev -v $PWD:/out --privileged autonomy/dianemo:latest image -l -f -p vmware -u file:///userdata.iso
docker run --rm -it -v /dev:/dev -v $PWD:/out --privileged autonomy/dianemo:latest image -l -f -p vmware -u http://192.168.1.100:8080/master.yaml
```

```bash
docker run --rm -it -v $PWD:/out --privileged autonomy/dianemo:latest vmdk
```

```bash
socat -d -d unix-connect:/tmp/serial0 stdio
```

```bash
screen /dev/ttys001
```

```bash
mkisofs -R -V config-2 -o userdata.iso userdata
```
