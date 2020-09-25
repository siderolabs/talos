<!-- markdownlint-disable -->
## talosctl cluster create

Creates a local docker-based or firecracker-based kubernetes cluster

### Synopsis

Creates a local docker-based or firecracker-based kubernetes cluster

```
talosctl cluster create [flags]
```

### Options

```
      --arch string                 cluster architecture (default "amd64")
      --cidr string                 CIDR of the cluster network (default "10.5.0.0/24")
      --cni-bin-path strings        search path for CNI binaries (VM only) (default [/opt/cni/bin])
      --cni-cache-dir string        CNI cache directory path (VM only) (default "/var/lib/cni")
      --cni-conf-dir string         CNI config directory path (VM only) (default "/etc/cni/conf.d")
      --cpus string                 the share of CPUs as fraction (each container/VM) (default "2.0")
      --crashdump                   print debug crashdump to stderr when cluster startup fails
      --custom-cni-url string       install custom CNI from the URL (Talos cluster)
      --disk int                    the limit on disk size in MB (each VM) (default 6144)
      --dns-domain string           the dns domain to use for cluster (default "cluster.local")
      --docker-host-ip string       Host IP to forward exposed ports to (Docker provisioner only) (default "0.0.0.0")
      --endpoint string             use endpoint instead of provider defaults
  -p, --exposed-ports string        Comma-separated list of ports/protocols to expose on init node. Ex -p <hostPort>:<containerPort>/<protocol (tcp or udp)> (Docker provisioner only)
  -h, --help                        help for create
      --image string                the image to use (default "ghcr.io/talos-systems/talos:latest")
      --init-node-as-endpoint       use init node as endpoint instead of any load balancer endpoint
      --initrd-path string          the uncompressed kernel image to use (default "_out/initramfs-${ARCH}.xz")
  -i, --input-dir string            location of pre-generated config files
      --install-image string        the installer image to use (default "ghcr.io/talos-systems/installer:latest")
      --kubernetes-version string   desired kubernetes version to run (default "1.19.1")
      --masters int                 the number of masters to create (default 1)
      --memory int                  the limit on memory usage in MB (each container/VM) (default 2048)
      --mtu int                     MTU of the cluster network (default 1500)
      --nameservers strings         list of nameservers to use (default [8.8.8.8,1.1.1.1])
      --registry-mirror strings     list of registry mirrors to use in format: <registry host>=<mirror URL>
      --vmlinuz-path string         the compressed kernel image to use (default "_out/vmlinuz-${ARCH}")
      --wait                        wait for the cluster to be ready before returning (default true)
      --wait-timeout duration       timeout to wait for the cluster to be ready (default 20m0s)
      --with-bootloader             enable bootloader to load kernel and initramfs from disk image after install (default true)
      --with-debug                  enable debug in Talos config to send service logs to the console
      --with-init-node              create the cluster with an init node
      --with-uefi                   enable UEFI on x86_64 architecture (always enabled for arm64)
      --workers int                 the number of workers to create (default 1)
```

### Options inherited from parent commands

```
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
      --name string          the name of the cluster (default "talos-default")
  -n, --nodes strings        target the specified nodes
      --provisioner string   Talos cluster provisioner to use (default "docker")
      --state string         directory path to store cluster state (default "/home/user/.talos/clusters")
      --talosconfig string   The path to the Talos configuration file (default "/home/user/.talos/config")
```

### SEE ALSO

* [talosctl cluster](talosctl_cluster.md)	 - A collection of commands for managing local docker-based or firecracker-based clusters

