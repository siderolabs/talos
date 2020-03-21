# Firecracker Talos Provisioner

This code is experimental for now.

Due to CNI, it requires `talosctl` to be running with at least
`CAP_SYS_ADMIN` and `CAP_NET_ADMIN` Linux capabilities
(in order to have the ability to create and configure network namespaces).

In any case, it requires `/dev/kvm` to be accessible for the user
running `talosctl`: https://github.com/firecracker-microvm/firecracker/blob/master/docs/getting-started.md#prerequisites

CNI configuration directory (could be overridden with `talosctl` flags) should
exist, default location is `/etc/cni/conf.d`.

Network namespace default mountpoint should be created as well: `/var/run/netns`.

Following CNI plugins should be installed to the CNI binary path (default is `/opt/cni/bin`):

- `bridge`
- `firewall`
- `tc-redirect-tap`

First two CNI plugins are part of [Standard CNI plugins](https://github.com/containernetworking/cni),
last one can be built from [Firecracker Go SDK](https://github.com/firecracker-microvm/firecracker-go-sdk/tree/master/cni).

Provisioner creates bridge interface with format `talos<8 hex chars>` and never deletes it (bug).
