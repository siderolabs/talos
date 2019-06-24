---
title: "osctl"
date: 2018-10-29T19:40:55-07:00
draft: false
menu:
  docs:
    parent: 'components'
---

`osctl` CLI is the client to the [osd](/components/osd) service running on every node. `osctl` should provide enough functionality to be a replacement for typical interactive shell operations. With it you can do things like:

- `osctl logs <service>` - retrieve container logs
- `osctl restart <service>` - restart a service
- `osctl reboot` - reset a node
- `osctl dmesg` - retrieve kernel logs
- `osctl ps` - view running services
- `osctl top` - view node resources
- `osctl services` - view status of talos services
