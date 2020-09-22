---
title: 'talosctl'
---

`talosctl` CLI is the client to the [apid](/components/apid) service running on every node.
`talosctl` should provide enough functionality to be a replacement for typical interactive shell operations.
With it you can do things like:

- `talosctl logs <service>` - retrieve container logs
- `talosctl restart <service>` - restart a service
- `talosctl reboot` - reset a node
- `talosctl dmesg` - retrieve kernel logs
- `talosctl ps` - view running services
- `talosctl top` - view node resources
- `talosctl services` - view status of Talos services
