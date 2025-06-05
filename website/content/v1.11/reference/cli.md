---
description: Talosctl CLI tool reference.
title: CLI
---

<!-- markdownlint-disable -->

## talosctl apply-config

Apply a new configuration to a node

```
talosctl apply-config [flags]
```

### Options

```
      --cert-fingerprint strings                                 list of server certificate fingeprints to accept (defaults to no check)
  -p, --config-patch stringArray                                 the list of config patches to apply to the local config file before sending it to the node
      --dry-run                                                  check how the config change will be applied in dry-run mode
  -f, --file string                                              the filename of the updated configuration
  -h, --help                                                     help for apply-config
  -i, --insecure                                                 apply the config using the insecure (encrypted with no auth) maintenance service
  -m, --mode auto, interactive, no-reboot, reboot, staged, try   apply config mode (default auto)
      --timeout duration                                         the config will be rolled back after specified timeout (if try mode is selected) (default 1m0s)
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl](#talosctl)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos

## talosctl bootstrap

Bootstrap the etcd cluster on the specified node.

### Synopsis

When Talos cluster is created etcd service on control plane nodes enter the join loop waiting
to join etcd peers from other control plane nodes. One node should be picked as the bootstrap node.
When bootstrap command is issued, the node aborts join process and bootstraps etcd cluster as a single node cluster.
Other control plane nodes will join etcd cluster once Kubernetes is bootstrapped on the bootstrap node.

This command should not be used when "init" type node are used.

Talos etcd cluster can be recovered from a known snapshot with '--recover-from=' flag.

```
talosctl bootstrap [flags]
```

### Options

```
  -h, --help                      help for bootstrap
      --recover-from string       recover etcd cluster from the snapshot
      --recover-skip-hash-check   skip integrity check when recovering etcd (use when recovering from data directory copy)
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl](#talosctl)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos

## talosctl cgroups

Retrieve cgroups usage information

### Synopsis

The cgroups command fetches control group v2 (cgroupv2) usage details from the machine.
Several presets are available to focus on specific cgroup subsystems:

* cpu
* cpuset
* io
* memory
* process
* swap

You can specify the preset using the --preset flag.

Alternatively, a custom schema can be provided using the --schema-file flag.
To see schema examples, refer to https://github.com/siderolabs/talos/tree/main/cmd/talosctl/cmd/talos/cgroupsprinter/schemas.


```
talosctl cgroups [flags]
```

### Options

```
  -h, --help                 help for cgroups
      --preset string        preset name (one of: [cpu cpuset io memory process swap])
      --schema-file string   path to the columns schema file
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl](#talosctl)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos

## talosctl cluster create

Creates a local docker-based or QEMU-based kubernetes cluster

```
talosctl cluster create [flags]
```

### Options

```
      --cidr string                              CIDR of the cluster network (IPv4, ULA network for IPv6 is derived in automated way) (default "10.5.0.0/24")
      --config-patch stringArray                 patch generated machineconfigs (applied to all node types), use @file to read a patch from file
      --config-patch-control-plane stringArray   patch generated machineconfigs (applied to 'init' and 'controlplane' types)
      --config-patch-worker stringArray          patch generated machineconfigs (applied to 'worker' type)
      --control-plane-port int                   control plane port (load balancer and local API port) (default 6443)
      --controlplanes int                        the number of controlplanes to create (default 1)
      --cpus string                              the share of CPUs as fraction (each control plane/VM) (default "2.0")
      --cpus-workers string                      the share of CPUs as fraction (each worker/VM) (default "2.0")
      --custom-cni-url string                    install custom CNI from the URL (Talos cluster)
      --dns-domain string                        the dns domain to use for cluster (default "cluster.local")
      --endpoint string                          use endpoint instead of provider defaults
      --init-node-as-endpoint                    use init node as endpoint instead of any load balancer endpoint
  -i, --input-dir string                         location of pre-generated config files
      --ipv4                                     enable IPv4 network in the cluster (default true)
      --kubeprism-port int                       KubePrism port (set to 0 to disable) (default 7445)
      --kubernetes-version string                desired kubernetes version to run (default "1.33.1")
      --memory int                               the limit on memory usage in MB (each control plane/VM) (default 2048)
      --memory-workers int                       the limit on memory usage in MB (each worker/VM) (default 2048)
      --mtu int                                  MTU of the cluster network (default 1500)
      --registry-insecure-skip-verify strings    list of registry hostnames to skip TLS verification for
      --registry-mirror strings                  list of registry mirrors to use in format: <registry host>=<mirror URL>
      --skip-injecting-config                    skip injecting config from embedded metadata server, write config files to current directory
      --skip-k8s-node-readiness-check            skip k8s node readiness checks
      --skip-kubeconfig                          skip merging kubeconfig from the created cluster
      --talos-version string                     the desired Talos version to generate config for (if not set, defaults to image version)
      --talosconfig string                       The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
      --wait                                     wait for the cluster to be ready before returning (default true)
      --wait-timeout duration                    timeout to wait for the cluster to be ready (default 20m0s)
      --wireguard-cidr string                    CIDR of the wireguard network
      --with-apply-config                        enable apply config when the VM is starting in maintenance mode
      --with-cluster-discovery                   enable cluster discovery (default true)
      --with-debug                               enable debug in Talos config to send service logs to the console
      --with-init-node                           create the cluster with an init node
      --with-json-logs                           enable JSON logs receiver and configure Talos to send logs there
      --with-kubespan                            enable KubeSpan system
      --workers int                              the number of workers to create (default 1)
      --arch string                              (qemu) cluster architecture (default "amd64")
      --bad-rtc                                  (qemu) launch VM with bad RTC state
      --cni-bin-path strings                     (qemu) search path for CNI binaries (default [/home/user/.talos/cni/bin])
      --cni-bundle-url string                    (qemu) URL to download CNI bundle from (default "https://github.com/siderolabs/talos/releases/download/v1.11.0-alpha.1/talosctl-cni-bundle-${ARCH}.tar.gz")
      --cni-cache-dir string                     (qemu) CNI cache directory path (default "/home/user/.talos/cni/cache")
      --cni-conf-dir string                      (qemu) CNI config directory path (default "/home/user/.talos/cni/conf.d")
      --config-injection-method string           (qemu) a method to inject machine config: default is HTTP server, 'metal-iso' to mount an ISO
      --disable-dhcp-hostname                    (qemu) skip announcing hostname via DHCP
      --disk int                                 (qemu) default limit on disk size in MB (each VM) (default 6144)
      --disk-block-size uint                     (qemu) disk block size (default 512)
      --disk-encryption-key-types stringArray    (qemu) encryption key types to use for disk encryption (uuid, kms) (default [uuid])
      --disk-image-path string                   (qemu) disk image to use
      --disk-preallocate                         (qemu) whether disk space should be preallocated (default true)
      --encrypt-ephemeral                        (qemu) enable ephemeral partition encryption
      --encrypt-state                            (qemu) enable state partition encryption
      --encrypt-user-volumes                     (qemu) enable ephemeral partition encryption
      --extra-boot-kernel-args string            (qemu) add extra kernel args to the initial boot from vmlinuz and initramfs
      --extra-disks int                          (qemu) number of extra disks to create for each worker VM
      --extra-disks-drivers strings              (qemu) driver for each extra disk (virtio, ide, ahci, scsi, nvme, megaraid)
      --extra-disks-size int                     (qemu) default limit on disk size in MB (each VM) (default 5120)
      --extra-uefi-search-paths strings          (qemu) additional search paths for UEFI firmware (only applies when UEFI is enabled)
      --initrd-path string                       (qemu) initramfs image to use (default "_out/initramfs-${ARCH}.xz")
      --install-image string                     (qemu) the installer image to use (default "ghcr.io/siderolabs/installer:latest")
      --ipv6                                     (qemu) enable IPv6 network in the cluster
      --ipxe-boot-script string                  (qemu) iPXE boot script (URL) to use
      --iso-path string                          (qemu) the ISO path to use for the initial boot
      --nameservers strings                      (qemu) list of nameservers to use (default [8.8.8.8,1.1.1.1,2001:4860:4860::8888,2606:4700:4700::1111])
      --no-masquerade-cidrs strings              (qemu) list of CIDRs to exclude from NAT
      --uki-path string                          (qemu) the UKI image path to use for the initial boot
      --usb-path string                          (qemu) the USB stick image path to use for the initial boot
      --use-vip                                  (qemu) use a virtual IP for the controlplane endpoint instead of the loadbalancer
      --user-volumes strings                     (qemu) list of user volumes to create for each VM in format: <name1>:<size1>:<name2>:<size2>
      --vmlinuz-path string                      (qemu) the compressed kernel image to use (default "_out/vmlinuz-${ARCH}")
      --with-bootloader                          (qemu) enable bootloader to load kernel and initramfs from disk image after install (default true)
      --with-firewall string                     (qemu) inject firewall rules into the cluster, value is default policy - accept/block
      --with-iommu                               (qemu) enable IOMMU support, this also add a new PCI root port and an interface attached to it
      --with-network-bandwidth int               (qemu) specify bandwidth restriction (in kbps) on the bridge interface
      --with-network-chaos                       (qemu) enable to use network chaos parameters
      --with-network-jitter duration             (qemu) specify jitter on the bridge interface
      --with-network-latency duration            (qemu) specify latency on the bridge interface
      --with-network-packet-corrupt float        (qemu) specify percent of corrupt packets on the bridge interface. e.g. 50% = 0.50 (default: 0.0)
      --with-network-packet-loss float           (qemu) specify percent of packet loss on the bridge interface. e.g. 50% = 0.50 (default: 0.0)
      --with-network-packet-reorder float        (qemu) specify percent of reordered packets on the bridge interface. e.g. 50% = 0.50 (default: 0.0)
      --with-siderolink true                     (qemu) enables the use of siderolink agent as configuration apply mechanism. true or `wireguard` enables the agent, `tunnel` enables the agent with grpc tunneling (default none)
      --with-tpm1_2                              (qemu) enable TPM 1.2 emulation support using swtpm
      --with-tpm2                                (qemu) enable TPM 2.0 emulation support using swtpm
      --with-uefi                                (qemu) enable UEFI on x86_64 architecture (default true)
      --with-uuid-hostnames                      (qemu) use machine UUIDs as default hostnames
      --docker-disable-ipv6                      (docker) skip enabling IPv6 in containers
      --docker-host-ip string                    (docker) Host IP to forward exposed ports to (default "0.0.0.0")
  -p, --exposed-ports string                     (docker) Comma-separated list of ports/protocols to expose on init node. Ex -p <hostPort>:<containerPort>/<protocol (tcp or udp)>
      --image string                             (docker) the image to use (default "ghcr.io/siderolabs/talos:latest")
      --mount mount                              (docker) attach a mount to the container
  -h, --help                                     help for create
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
      --name string          the name of the cluster (default "talos-default")
  -n, --nodes strings        target the specified nodes
      --provisioner string   Talos cluster provisioner to use (default "docker")
      --state string         directory path to store cluster state (default "/home/user/.talos/clusters")
```

### SEE ALSO

* [talosctl cluster](#talosctl-cluster)	 - A collection of commands for managing local docker-based or QEMU-based clusters

## talosctl cluster destroy

Destroys a local docker-based or firecracker-based kubernetes cluster

```
talosctl cluster destroy [flags]
```

### Options

```
  -f, --force                                   force deletion of cluster directory if there were errors
  -h, --help                                    help for destroy
      --save-cluster-logs-archive-path string   save cluster logs archive to the specified file on destroy
      --save-support-archive-path string        save support archive to the specified file on destroy
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
      --name string          the name of the cluster (default "talos-default")
  -n, --nodes strings        target the specified nodes
      --provisioner string   Talos cluster provisioner to use (default "docker")
      --state string         directory path to store cluster state (default "/home/user/.talos/clusters")
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl cluster](#talosctl-cluster)	 - A collection of commands for managing local docker-based or QEMU-based clusters

## talosctl cluster show

Shows info about a local provisioned kubernetes cluster

```
talosctl cluster show [flags]
```

### Options

```
  -h, --help   help for show
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
      --name string          the name of the cluster (default "talos-default")
  -n, --nodes strings        target the specified nodes
      --provisioner string   Talos cluster provisioner to use (default "docker")
      --state string         directory path to store cluster state (default "/home/user/.talos/clusters")
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl cluster](#talosctl-cluster)	 - A collection of commands for managing local docker-based or QEMU-based clusters

## talosctl cluster

A collection of commands for managing local docker-based or QEMU-based clusters

### Options

```
  -h, --help                 help for cluster
      --name string          the name of the cluster (default "talos-default")
      --provisioner string   Talos cluster provisioner to use (default "docker")
      --state string         directory path to store cluster state (default "/home/user/.talos/clusters")
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl](#talosctl)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos
* [talosctl cluster create](#talosctl-cluster-create)	 - Creates a local docker-based or QEMU-based kubernetes cluster
* [talosctl cluster destroy](#talosctl-cluster-destroy)	 - Destroys a local docker-based or firecracker-based kubernetes cluster
* [talosctl cluster show](#talosctl-cluster-show)	 - Shows info about a local provisioned kubernetes cluster

## talosctl completion

Output shell completion code for the specified shell (bash, fish or zsh)

### Synopsis

Output shell completion code for the specified shell (bash, fish or zsh).
The shell code must be evaluated to provide interactive
completion of talosctl commands.  This can be done by sourcing it from
the .bash_profile.

Note for zsh users: [1] zsh completions are only supported in versions of zsh >= 5.2

```
talosctl completion SHELL [flags]
```

### Examples

```
# Installing bash completion on macOS using homebrew
## If running Bash 3.2 included with macOS
	brew install bash-completion
## or, if running Bash 4.1+
	brew install bash-completion@2
## If talosctl is installed via homebrew, this should start working immediately.
## If you've installed via other means, you may need add the completion to your completion directory
	talosctl completion bash > $(brew --prefix)/etc/bash_completion.d/talosctl

# Installing bash completion on Linux
## If bash-completion is not installed on Linux, please install the 'bash-completion' package
## via your distribution's package manager.
## Load the talosctl completion code for bash into the current shell
	source <(talosctl completion bash)
## Write bash completion code to a file and source if from .bash_profile
	talosctl completion bash > ~/.talos/completion.bash.inc
	printf "
		# talosctl shell completion
		source '$HOME/.talos/completion.bash.inc'
		" >> $HOME/.bash_profile
	source $HOME/.bash_profile
# Load the talosctl completion code for fish[1] into the current shell
	talosctl completion fish | source
# Set the talosctl completion code for fish[1] to autoload on startup
    talosctl completion fish > ~/.config/fish/completions/talosctl.fish
# Load the talosctl completion code for zsh[1] into the current shell
	source <(talosctl completion zsh)
# Set the talosctl completion code for zsh[1] to autoload on startup
    talosctl completion zsh > "${fpath[1]}/_talosctl"
```

### Options

```
  -h, --help   help for completion
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl](#talosctl)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos

## talosctl config add

Add a new context

```
talosctl config add <context> [flags]
```

### Options

```
      --ca string    the path to the CA certificate
      --crt string   the path to the certificate
  -h, --help         help for add
      --key string   the path to the key
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl config](#talosctl-config)	 - Manage the client configuration file (talosconfig)

## talosctl config context

Set the current context

```
talosctl config context <context> [flags]
```

### Options

```
  -h, --help   help for context
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl config](#talosctl-config)	 - Manage the client configuration file (talosconfig)

## talosctl config contexts

List defined contexts

```
talosctl config contexts [flags]
```

### Options

```
  -h, --help   help for contexts
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl config](#talosctl-config)	 - Manage the client configuration file (talosconfig)

## talosctl config endpoint

Set the endpoint(s) for the current context

```
talosctl config endpoint <endpoint>... [flags]
```

### Options

```
  -h, --help   help for endpoint
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl config](#talosctl-config)	 - Manage the client configuration file (talosconfig)

## talosctl config info

Show information about the current context

```
talosctl config info [flags]
```

### Options

```
  -h, --help            help for info
  -o, --output string   output format (json|yaml|text). Default text. (default "text")
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl config](#talosctl-config)	 - Manage the client configuration file (talosconfig)

## talosctl config merge

Merge additional contexts from another client configuration file

### Synopsis

Contexts with the same name are renamed while merging configs.

```
talosctl config merge <from> [flags]
```

### Options

```
  -h, --help   help for merge
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl config](#talosctl-config)	 - Manage the client configuration file (talosconfig)

## talosctl config new

Generate a new client configuration file

```
talosctl config new [<path>] [flags]
```

### Options

```
      --crt-ttl duration   certificate TTL (default 8760h0m0s)
  -h, --help               help for new
      --roles strings      roles (default [os:admin])
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl config](#talosctl-config)	 - Manage the client configuration file (talosconfig)

## talosctl config node

Set the node(s) for the current context

```
talosctl config node <endpoint>... [flags]
```

### Options

```
  -h, --help   help for node
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl config](#talosctl-config)	 - Manage the client configuration file (talosconfig)

## talosctl config remove

Remove contexts

```
talosctl config remove <context> [flags]
```

### Options

```
      --dry-run     dry run
  -h, --help        help for remove
  -y, --noconfirm   do not ask for confirmation
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl config](#talosctl-config)	 - Manage the client configuration file (talosconfig)

## talosctl config

Manage the client configuration file (talosconfig)

### Options

```
  -h, --help   help for config
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl](#talosctl)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos
* [talosctl config add](#talosctl-config-add)	 - Add a new context
* [talosctl config context](#talosctl-config-context)	 - Set the current context
* [talosctl config contexts](#talosctl-config-contexts)	 - List defined contexts
* [talosctl config endpoint](#talosctl-config-endpoint)	 - Set the endpoint(s) for the current context
* [talosctl config info](#talosctl-config-info)	 - Show information about the current context
* [talosctl config merge](#talosctl-config-merge)	 - Merge additional contexts from another client configuration file
* [talosctl config new](#talosctl-config-new)	 - Generate a new client configuration file
* [talosctl config node](#talosctl-config-node)	 - Set the node(s) for the current context
* [talosctl config remove](#talosctl-config-remove)	 - Remove contexts

## talosctl conformance kubernetes

Run Kubernetes conformance tests

```
talosctl conformance kubernetes [flags]
```

### Options

```
  -h, --help          help for kubernetes
      --mode string   conformance test mode: [fast, certified] (default "fast")
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl conformance](#talosctl-conformance)	 - Run conformance tests

## talosctl conformance

Run conformance tests

### Options

```
  -h, --help   help for conformance
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl](#talosctl)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos
* [talosctl conformance kubernetes](#talosctl-conformance-kubernetes)	 - Run Kubernetes conformance tests

## talosctl containers

List containers

```
talosctl containers [flags]
```

### Options

```
  -h, --help         help for containers
  -k, --kubernetes   use the k8s.io containerd namespace
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl](#talosctl)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos

## talosctl copy

Copy data out from the node

### Synopsis

Creates an .tar.gz archive at the node starting at <src-path> and
streams it back to the client.

If '-' is given for <local-path>, archive is written to stdout.
Otherwise archive is extracted to <local-path> which should be an empty directory or
talosctl creates a directory if <local-path> doesn't exist. Command doesn't preserve
ownership and access mode for the files in extract mode, while  streamed .tar archive
captures ownership and permission bits.

```
talosctl copy <src-path> -|<local-path> [flags]
```

### Options

```
  -h, --help   help for copy
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl](#talosctl)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos

## talosctl dashboard

Cluster dashboard with node overview, logs and real-time metrics

### Synopsis

Provide a text-based UI to navigate node overview, logs and real-time metrics.

Keyboard shortcuts:

 - h, <Left> - switch one node to the left
 - l, <Right> - switch one node to the right
 - j, <Down> - scroll logs/process list down
 - k, <Up> - scroll logs/process list up
 - <C-d> - scroll logs/process list half page down
 - <C-u> - scroll logs/process list half page up
 - <C-f> - scroll logs/process list one page down
 - <C-b> - scroll logs/process list one page up


```
talosctl dashboard [flags]
```

### Options

```
  -h, --help                       help for dashboard
  -d, --update-interval duration   interval between updates (default 3s)
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl](#talosctl)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos

## talosctl dmesg

Retrieve kernel logs

```
talosctl dmesg [flags]
```

### Options

```
  -f, --follow   specify if the kernel log should be streamed
  -h, --help     help for dmesg
      --tail     specify if only new messages should be sent (makes sense only when combined with --follow)
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl](#talosctl)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos

## talosctl edit

Edit a resource from the default editor.

### Synopsis

The edit command allows you to directly edit any API resource
you can retrieve via the command line tools.

It will open the editor defined by your TALOS_EDITOR,
or EDITOR environment variables, or fall back to 'vi' for Linux
or 'notepad' for Windows.

```
talosctl edit <type> [<id>] [flags]
```

### Options

```
      --dry-run                                     do not apply the change after editing and print the change summary instead
  -h, --help                                        help for edit
  -m, --mode auto, no-reboot, reboot, staged, try   apply config mode (default auto)
      --namespace string                            resource namespace (default is to use default namespace per resource)
      --timeout duration                            the config will be rolled back after specified timeout (if try mode is selected) (default 1m0s)
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl](#talosctl)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos

## talosctl etcd alarm disarm

Disarm the etcd alarms for the node.

```
talosctl etcd alarm disarm [flags]
```

### Options

```
  -h, --help   help for disarm
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl etcd alarm](#talosctl-etcd-alarm)	 - Manage etcd alarms

## talosctl etcd alarm list

List the etcd alarms for the node.

```
talosctl etcd alarm list [flags]
```

### Options

```
  -h, --help   help for list
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl etcd alarm](#talosctl-etcd-alarm)	 - Manage etcd alarms

## talosctl etcd alarm

Manage etcd alarms

### Options

```
  -h, --help   help for alarm
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl etcd](#talosctl-etcd)	 - Manage etcd
* [talosctl etcd alarm disarm](#talosctl-etcd-alarm-disarm)	 - Disarm the etcd alarms for the node.
* [talosctl etcd alarm list](#talosctl-etcd-alarm-list)	 - List the etcd alarms for the node.

## talosctl etcd defrag

Defragment etcd database on the node

### Synopsis

Defragmentation is a maintenance operation that releases unused space from the etcd database file.
Defragmentation is a resource heavy operation and should be performed only when necessary on a single node at a time.

```
talosctl etcd defrag [flags]
```

### Options

```
  -h, --help   help for defrag
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl etcd](#talosctl-etcd)	 - Manage etcd

## talosctl etcd forfeit-leadership

Tell node to forfeit etcd cluster leadership

```
talosctl etcd forfeit-leadership [flags]
```

### Options

```
  -h, --help   help for forfeit-leadership
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl etcd](#talosctl-etcd)	 - Manage etcd

## talosctl etcd leave

Tell nodes to leave etcd cluster

```
talosctl etcd leave [flags]
```

### Options

```
  -h, --help   help for leave
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl etcd](#talosctl-etcd)	 - Manage etcd

## talosctl etcd members

Get the list of etcd cluster members

```
talosctl etcd members [flags]
```

### Options

```
  -h, --help   help for members
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl etcd](#talosctl-etcd)	 - Manage etcd

## talosctl etcd remove-member

Remove the node from etcd cluster

### Synopsis

Use this command only if you want to remove a member which is in broken state.
If there is no access to the node, or the node can't access etcd to call etcd leave.
Always prefer etcd leave over this command.

```
talosctl etcd remove-member <member ID> [flags]
```

### Options

```
  -h, --help   help for remove-member
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl etcd](#talosctl-etcd)	 - Manage etcd

## talosctl etcd snapshot

Stream snapshot of the etcd node to the path.

```
talosctl etcd snapshot <path> [flags]
```

### Options

```
  -h, --help   help for snapshot
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl etcd](#talosctl-etcd)	 - Manage etcd

## talosctl etcd status

Get the status of etcd cluster member

### Synopsis

Returns the status of etcd member on the node, use multiple nodes to get status of all members.

```
talosctl etcd status [flags]
```

### Options

```
  -h, --help   help for status
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl etcd](#talosctl-etcd)	 - Manage etcd

## talosctl etcd

Manage etcd

### Options

```
  -h, --help   help for etcd
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl](#talosctl)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos
* [talosctl etcd alarm](#talosctl-etcd-alarm)	 - Manage etcd alarms
* [talosctl etcd defrag](#talosctl-etcd-defrag)	 - Defragment etcd database on the node
* [talosctl etcd forfeit-leadership](#talosctl-etcd-forfeit-leadership)	 - Tell node to forfeit etcd cluster leadership
* [talosctl etcd leave](#talosctl-etcd-leave)	 - Tell nodes to leave etcd cluster
* [talosctl etcd members](#talosctl-etcd-members)	 - Get the list of etcd cluster members
* [talosctl etcd remove-member](#talosctl-etcd-remove-member)	 - Remove the node from etcd cluster
* [talosctl etcd snapshot](#talosctl-etcd-snapshot)	 - Stream snapshot of the etcd node to the path.
* [talosctl etcd status](#talosctl-etcd-status)	 - Get the status of etcd cluster member

## talosctl events

Stream runtime events

```
talosctl events [flags]
```

### Options

```
      --actor-id string     filter events by the specified actor ID (default is no filter)
      --duration duration   show events for the past duration interval (one second resolution, default is to show no history)
  -h, --help                help for events
      --since string        show events after the specified event ID (default is to show no history)
      --tail int32          show specified number of past events (use -1 to show full history, default is to show no history)
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl](#talosctl)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos

## talosctl gen ca

Generates a self-signed X.509 certificate authority

```
talosctl gen ca [flags]
```

### Options

```
  -h, --help                  help for ca
      --hours int             the hours from now on which the certificate validity period ends (default 87600)
      --organization string   X.509 distinguished name for the Organization
      --rsa                   generate in RSA format
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -f, --force                will overwrite existing files
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl gen](#talosctl-gen)	 - Generate CAs, certificates, and private keys

## talosctl gen config

Generates a set of configuration files for Talos cluster

### Synopsis

The cluster endpoint is the URL for the Kubernetes API. If you decide to use
a control plane node, common in a single node control plane setup, use port 6443 as
this is the port that the API server binds to on every control plane node. For an HA
setup, usually involving a load balancer, use the IP and port of the load balancer.

```
talosctl gen config <cluster name> <cluster endpoint> [flags]
```

### Options

```
      --additional-sans strings                  additional Subject-Alt-Names for the APIServer certificate
      --config-patch stringArray                 patch generated machineconfigs (applied to all node types), use @file to read a patch from file
      --config-patch-control-plane stringArray   patch generated machineconfigs (applied to 'init' and 'controlplane' types)
      --config-patch-worker stringArray          patch generated machineconfigs (applied to 'worker' type)
      --dns-domain string                        the dns domain to use for cluster (default "cluster.local")
  -h, --help                                     help for config
      --install-disk string                      the disk to install to (default "/dev/sda")
      --install-image string                     the image used to perform an installation (default "ghcr.io/siderolabs/installer:latest")
      --kubernetes-version string                desired kubernetes version to run (default "1.33.1")
  -o, --output string                            destination to output generated files. when multiple output types are specified, it must be a directory. for a single output type, it must either be a file path, or "-" for stdout
  -t, --output-types strings                     types of outputs to be generated. valid types are: ["controlplane" "worker" "talosconfig"] (default [controlplane,worker,talosconfig])
  -p, --persist                                  the desired persist value for configs (default true)
      --registry-mirror strings                  list of registry mirrors to use in format: <registry host>=<mirror URL>
      --talos-version string                     the desired Talos version to generate config for (backwards compatibility, e.g. v0.8)
      --version string                           the desired machine config version to generate (default "v1alpha1")
      --with-cluster-discovery                   enable cluster discovery feature (default true)
      --with-docs                                renders all machine configs adding the documentation for each field (default true)
      --with-examples                            renders all machine configs with the commented examples (default true)
      --with-kubespan                            enable KubeSpan feature
      --with-secrets string                      use a secrets file generated using 'gen secrets'
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -f, --force                will overwrite existing files
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl gen](#talosctl-gen)	 - Generate CAs, certificates, and private keys

## talosctl gen crt

Generates an X.509 Ed25519 certificate

```
talosctl gen crt [flags]
```

### Options

```
      --ca string     path to the PEM encoded CERTIFICATE
      --csr string    path to the PEM encoded CERTIFICATE REQUEST
  -h, --help          help for crt
      --hours int     the hours from now on which the certificate validity period ends (default 24)
      --name string   the basename of the generated file
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -f, --force                will overwrite existing files
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl gen](#talosctl-gen)	 - Generate CAs, certificates, and private keys

## talosctl gen csr

Generates a CSR using an Ed25519 private key

```
talosctl gen csr [flags]
```

### Options

```
  -h, --help            help for csr
      --ip string       generate the certificate for this IP address
      --key string      path to the PEM encoded EC or RSA PRIVATE KEY
      --roles strings   roles (default [os:admin])
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -f, --force                will overwrite existing files
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl gen](#talosctl-gen)	 - Generate CAs, certificates, and private keys

## talosctl gen key

Generates an Ed25519 private key

```
talosctl gen key [flags]
```

### Options

```
  -h, --help          help for key
      --name string   the basename of the generated file
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -f, --force                will overwrite existing files
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl gen](#talosctl-gen)	 - Generate CAs, certificates, and private keys

## talosctl gen keypair

Generates an X.509 Ed25519 key pair

```
talosctl gen keypair [flags]
```

### Options

```
  -h, --help                  help for keypair
      --ip string             generate the certificate for this IP address
      --organization string   X.509 distinguished name for the Organization
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -f, --force                will overwrite existing files
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl gen](#talosctl-gen)	 - Generate CAs, certificates, and private keys

## talosctl gen secrets

Generates a secrets bundle file which can later be used to generate a config

```
talosctl gen secrets [flags]
```

### Options

```
      --from-controlplane-config string     use the provided controlplane Talos machine configuration as input
  -p, --from-kubernetes-pki string          use a Kubernetes PKI directory (e.g. /etc/kubernetes/pki) as input
  -h, --help                                help for secrets
  -t, --kubernetes-bootstrap-token string   use the provided bootstrap token as input
  -o, --output-file string                  path of the output file (default "secrets.yaml")
      --talos-version string                the desired Talos version to generate secrets bundle for (backwards compatibility, e.g. v0.8)
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -f, --force                will overwrite existing files
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl gen](#talosctl-gen)	 - Generate CAs, certificates, and private keys

## talosctl gen secureboot database

Generates a UEFI database to enroll the signing certificate

```
talosctl gen secureboot database [flags]
```

### Options

```
      --enrolled-certificate string     path to the certificate to enroll (default "_out/uki-signing-cert.pem")
  -h, --help                            help for database
      --include-well-known-uefi-certs   include well-known UEFI (Microsoft) certificates in the database
      --signing-certificate string      path to the certificate used to sign the database (default "_out/uki-signing-cert.pem")
      --signing-key string              path to the key used to sign the database (default "_out/uki-signing-key.pem")
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -f, --force                will overwrite existing files
  -n, --nodes strings        target the specified nodes
  -o, --output string        path to the directory storing the generated files (default "_out")
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl gen secureboot](#talosctl-gen-secureboot)	 - Generates secrets for the SecureBoot process

## talosctl gen secureboot pcr

Generates a key which is used to sign TPM PCR values

```
talosctl gen secureboot pcr [flags]
```

### Options

```
  -h, --help   help for pcr
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -f, --force                will overwrite existing files
  -n, --nodes strings        target the specified nodes
  -o, --output string        path to the directory storing the generated files (default "_out")
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl gen secureboot](#talosctl-gen-secureboot)	 - Generates secrets for the SecureBoot process

## talosctl gen secureboot uki

Generates a certificate which is used to sign boot assets (UKI)

```
talosctl gen secureboot uki [flags]
```

### Options

```
      --common-name string   common name for the certificate (default "Test UKI Signing Key")
  -h, --help                 help for uki
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -f, --force                will overwrite existing files
  -n, --nodes strings        target the specified nodes
  -o, --output string        path to the directory storing the generated files (default "_out")
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl gen secureboot](#talosctl-gen-secureboot)	 - Generates secrets for the SecureBoot process

## talosctl gen secureboot

Generates secrets for the SecureBoot process

### Options

```
  -h, --help            help for secureboot
  -o, --output string   path to the directory storing the generated files (default "_out")
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -f, --force                will overwrite existing files
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl gen](#talosctl-gen)	 - Generate CAs, certificates, and private keys
* [talosctl gen secureboot database](#talosctl-gen-secureboot-database)	 - Generates a UEFI database to enroll the signing certificate
* [talosctl gen secureboot pcr](#talosctl-gen-secureboot-pcr)	 - Generates a key which is used to sign TPM PCR values
* [talosctl gen secureboot uki](#talosctl-gen-secureboot-uki)	 - Generates a certificate which is used to sign boot assets (UKI)

## talosctl gen

Generate CAs, certificates, and private keys

### Options

```
  -f, --force   will overwrite existing files
  -h, --help    help for gen
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl](#talosctl)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos
* [talosctl gen ca](#talosctl-gen-ca)	 - Generates a self-signed X.509 certificate authority
* [talosctl gen config](#talosctl-gen-config)	 - Generates a set of configuration files for Talos cluster
* [talosctl gen crt](#talosctl-gen-crt)	 - Generates an X.509 Ed25519 certificate
* [talosctl gen csr](#talosctl-gen-csr)	 - Generates a CSR using an Ed25519 private key
* [talosctl gen key](#talosctl-gen-key)	 - Generates an Ed25519 private key
* [talosctl gen keypair](#talosctl-gen-keypair)	 - Generates an X.509 Ed25519 key pair
* [talosctl gen secrets](#talosctl-gen-secrets)	 - Generates a secrets bundle file which can later be used to generate a config
* [talosctl gen secureboot](#talosctl-gen-secureboot)	 - Generates secrets for the SecureBoot process

## talosctl get

Get a specific resource or list of resources (use 'talosctl get rd' to see all available resource types).

### Synopsis

Similar to 'kubectl get', 'talosctl get' returns a set of resources from the OS.
To get a list of all available resource definitions, issue 'talosctl get rd'

```
talosctl get <type> [<id>] [flags]
```

### Options

```
  -h, --help               help for get
  -i, --insecure           get resources using the insecure (encrypted with no auth) maintenance service
      --namespace string   resource namespace (default is to use default namespace per resource)
  -o, --output string      output mode (json, table, yaml, jsonpath) (default "table")
  -w, --watch              watch resource changes
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl](#talosctl)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos

## talosctl health

Check cluster health

```
talosctl health [flags]
```

### Options

```
      --control-plane-nodes strings   specify IPs of control plane nodes
  -h, --help                          help for health
      --init-node string              specify IPs of init node
      --k8s-endpoint string           use endpoint instead of kubeconfig default
      --run-e2e                       run Kubernetes e2e test
      --server                        run server-side check (default true)
      --wait-timeout duration         timeout to wait for the cluster to be ready (default 20m0s)
      --worker-nodes strings          specify IPs of worker nodes
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl](#talosctl)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos

## talosctl image cache-create

Create a cache of images in OCI format into a directory

### Synopsis

Create a cache of images in OCI format into a directory

```
talosctl image cache-create [flags]
```

### Examples

```
talosctl images cache-create --images=ghcr.io/siderolabs/kubelet:v1.33.1 --image-cache-path=/tmp/talos-image-cache

Alternatively, stdin can be piped to the command:
talosctl images default | talosctl images cache-create --image-cache-path=/tmp/talos-image-cache --images=-

```

### Options

```
      --force                           force overwrite of existing image cache
  -h, --help                            help for cache-create
      --image-cache-path string         directory to save the image cache in OCI format
      --image-layer-cache-path string   directory to save the image layer cache
      --images strings                  images to cache
      --insecure                        allow insecure registries
      --platform string                 platform to use for the cache (default "linux/amd64")
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
      --namespace system     namespace to use: system (etcd and kubelet images) or `cri` for all Kubernetes workloads (default "cri")
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl image](#talosctl-image)	 - Manage CRI container images

## talosctl image default

List the default images used by Talos

```
talosctl image default [flags]
```

### Options

```
  -h, --help   help for default
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
      --namespace system     namespace to use: system (etcd and kubelet images) or `cri` for all Kubernetes workloads (default "cri")
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl image](#talosctl-image)	 - Manage CRI container images

## talosctl image list

List CRI images

```
talosctl image list [flags]
```

### Options

```
  -h, --help   help for list
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
      --namespace system     namespace to use: system (etcd and kubelet images) or `cri` for all Kubernetes workloads (default "cri")
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl image](#talosctl-image)	 - Manage CRI container images

## talosctl image pull

Pull an image into CRI

```
talosctl image pull <image> [flags]
```

### Options

```
  -h, --help   help for pull
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
      --namespace system     namespace to use: system (etcd and kubelet images) or `cri` for all Kubernetes workloads (default "cri")
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl image](#talosctl-image)	 - Manage CRI container images

## talosctl image

Manage CRI container images

### Options

```
  -h, --help               help for image
      --namespace system   namespace to use: system (etcd and kubelet images) or `cri` for all Kubernetes workloads (default "cri")
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl](#talosctl)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos
* [talosctl image cache-create](#talosctl-image-cache-create)	 - Create a cache of images in OCI format into a directory
* [talosctl image default](#talosctl-image-default)	 - List the default images used by Talos
* [talosctl image list](#talosctl-image-list)	 - List CRI images
* [talosctl image pull](#talosctl-image-pull)	 - Pull an image into CRI

## talosctl inject serviceaccount

Inject Talos API ServiceAccount into Kubernetes manifests

```
talosctl inject serviceaccount [--roles='<ROLE_1>,<ROLE_2>'] -f <manifest.yaml> [flags]
```

### Examples

```
talosctl inject serviceaccount --roles="os:admin" -f deployment.yaml > deployment-injected.yaml

Alternatively, stdin can be piped to the command:
cat deployment.yaml | talosctl inject serviceaccount --roles="os:admin" -f - > deployment-injected.yaml

```

### Options

```
  -f, --file string     file with Kubernetes manifests to be injected with ServiceAccount
  -h, --help            help for serviceaccount
  -r, --roles strings   roles to add to the generated ServiceAccount manifests (default [os:reader])
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl inject](#talosctl-inject)	 - Inject Talos API resources into Kubernetes manifests

## talosctl inject

Inject Talos API resources into Kubernetes manifests

### Options

```
  -h, --help   help for inject
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl](#talosctl)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos
* [talosctl inject serviceaccount](#talosctl-inject-serviceaccount)	 - Inject Talos API ServiceAccount into Kubernetes manifests

## talosctl inspect dependencies

Inspect controller-resource dependencies as graphviz graph.

### Synopsis

Inspect controller-resource dependencies as graphviz graph.

Pipe the output of the command through the "dot" program (part of graphviz package)
to render the graph:

    talosctl inspect dependencies | dot -Tpng > graph.png


```
talosctl inspect dependencies [flags]
```

### Options

```
  -h, --help             help for dependencies
      --with-resources   display live resource information with dependencies
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl inspect](#talosctl-inspect)	 - Inspect internals of Talos

## talosctl inspect

Inspect internals of Talos

### Options

```
  -h, --help   help for inspect
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl](#talosctl)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos
* [talosctl inspect dependencies](#talosctl-inspect-dependencies)	 - Inspect controller-resource dependencies as graphviz graph.

## talosctl kubeconfig

Download the admin kubeconfig from the node

### Synopsis

Download the admin kubeconfig from the node.
If merge flag is true, config will be merged with ~/.kube/config or [local-path] if specified.
Otherwise, kubeconfig will be written to PWD or [local-path] if specified.

If merge flag is false and [local-path] is "-", config will be written to stdout.

```
talosctl kubeconfig [local-path] [flags]
```

### Options

```
  -f, --force                       Force overwrite of kubeconfig if already present, force overwrite on kubeconfig merge
      --force-context-name string   Force context name for kubeconfig merge
  -h, --help                        help for kubeconfig
  -m, --merge                       Merge with existing kubeconfig (default true)
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl](#talosctl)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos

## talosctl list

Retrieve a directory listing

```
talosctl list [path] [flags]
```

### Options

```
  -d, --depth int32    maximum recursion depth (default 1)
  -h, --help           help for list
  -H, --humanize       humanize size and time in the output
  -l, --long           display additional file details
  -r, --recurse        recurse into subdirectories
  -t, --type strings   filter by specified types:
                       f	regular file
                       d	directory
                       l, L	symbolic link
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl](#talosctl)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos

## talosctl logs

Retrieve logs for a service

```
talosctl logs <service name> [flags]
```

### Options

```
  -f, --follow       specify if the logs should be streamed
  -h, --help         help for logs
  -k, --kubernetes   use the k8s.io containerd namespace
      --tail int32   lines of log file to display (default is to show from the beginning) (default -1)
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl](#talosctl)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos

## talosctl machineconfig gen

Generates a set of configuration files for Talos cluster

### Synopsis

The cluster endpoint is the URL for the Kubernetes API. If you decide to use
a control plane node, common in a single node control plane setup, use port 6443 as
this is the port that the API server binds to on every control plane node. For an HA
setup, usually involving a load balancer, use the IP and port of the load balancer.

```
talosctl machineconfig gen <cluster name> <cluster endpoint> [flags]
```

### Options

```
  -h, --help   help for gen
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl machineconfig](#talosctl-machineconfig)	 - Machine config related commands

## talosctl machineconfig patch

Patch a machine config

```
talosctl machineconfig patch <machineconfig-file> [flags]
```

### Options

```
  -h, --help                help for patch
  -o, --output string       output destination. if not specified, output will be printed to stdout
  -p, --patch stringArray   patch generated machineconfigs (applied to all node types), use @file to read a patch from file
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl machineconfig](#talosctl-machineconfig)	 - Machine config related commands

## talosctl machineconfig

Machine config related commands

### Options

```
  -h, --help   help for machineconfig
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl](#talosctl)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos
* [talosctl machineconfig gen](#talosctl-machineconfig-gen)	 - Generates a set of configuration files for Talos cluster
* [talosctl machineconfig patch](#talosctl-machineconfig-patch)	 - Patch a machine config

## talosctl memory

Show memory usage

```
talosctl memory [flags]
```

### Options

```
  -h, --help      help for memory
  -v, --verbose   display extended memory statistics
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl](#talosctl)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos

## talosctl meta delete

Delete a key from the META partition.

```
talosctl meta delete key [flags]
```

### Options

```
  -h, --help   help for delete
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -i, --insecure             write|delete meta using the insecure (encrypted with no auth) maintenance service
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl meta](#talosctl-meta)	 - Write and delete keys in the META partition

## talosctl meta write

Write a key-value pair to the META partition.

```
talosctl meta write key value [flags]
```

### Options

```
  -h, --help   help for write
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -i, --insecure             write|delete meta using the insecure (encrypted with no auth) maintenance service
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl meta](#talosctl-meta)	 - Write and delete keys in the META partition

## talosctl meta

Write and delete keys in the META partition

### Options

```
  -h, --help       help for meta
  -i, --insecure   write|delete meta using the insecure (encrypted with no auth) maintenance service
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl](#talosctl)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos
* [talosctl meta delete](#talosctl-meta-delete)	 - Delete a key from the META partition.
* [talosctl meta write](#talosctl-meta-write)	 - Write a key-value pair to the META partition.

## talosctl mounts

List mounts

```
talosctl mounts [flags]
```

### Options

```
  -h, --help   help for mounts
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl](#talosctl)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos

## talosctl netstat

Show network connections and sockets

### Synopsis

Show network connections and sockets.

You can pass an optional argument to view a specific pod's connections.
To do this, format the argument as "namespace/pod".
Note that only pods with a pod network namespace are allowed.
If you don't pass an argument, the command will show host connections.

```
talosctl netstat [flags]
```

### Options

```
  -a, --all         display all sockets states (default: connected)
  -x, --extend      show detailed socket information
  -h, --help        help for netstat
  -4, --ipv4        display only ipv4 sockets
  -6, --ipv6        display only ipv6 sockets
  -l, --listening   display listening server sockets
  -k, --pods        show sockets used by Kubernetes pods
  -p, --programs    show process using socket
  -w, --raw         display only RAW sockets
  -t, --tcp         display only TCP sockets
  -o, --timers      display timers
  -u, --udp         display only UDP sockets
  -U, --udplite     display only UDPLite sockets
  -v, --verbose     display sockets of all supported transport protocols
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl](#talosctl)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos

## talosctl patch

Update field(s) of a resource using a JSON patch.

```
talosctl patch <type> [<id>] [flags]
```

### Options

```
      --dry-run                                     print the change summary and patch preview without applying the changes
  -h, --help                                        help for patch
  -m, --mode auto, no-reboot, reboot, staged, try   apply config mode (default auto)
      --namespace string                            resource namespace (default is to use default namespace per resource)
  -p, --patch stringArray                           the patch to be applied to the resource file, use @file to read a patch from file.
      --patch-file string                           a file containing a patch to be applied to the resource.
      --timeout duration                            the config will be rolled back after specified timeout (if try mode is selected) (default 1m0s)
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl](#talosctl)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos

## talosctl pcap

Capture the network packets from the node.

### Synopsis

The command launches packet capture on the node and streams back the packets as raw pcap file.

Default behavior is to decode the packets with internal decoder to stdout:

    talosctl pcap -i eth0

Raw pcap file can be saved with `--output` flag:

    talosctl pcap -i eth0 --output eth0.pcap

Output can be piped to tcpdump:

    talosctl pcap -i eth0 -o - | tcpdump -vvv -r -

BPF filter can be applied, but it has to compiled to BPF instructions first using tcpdump.
Correct link type should be specified for the tcpdump: EN10MB for Ethernet links and RAW
for e.g. Wireguard tunnels:

    talosctl pcap -i eth0 --bpf-filter "$(tcpdump -dd -y EN10MB 'tcp and dst port 80')"

    talosctl pcap -i kubespan --bpf-filter "$(tcpdump -dd -y RAW 'port 50000')"

As packet capture is transmitted over the network, it is recommended to filter out the Talos API traffic,
e.g. by excluding packets with the port 50000.
   

```
talosctl pcap [flags]
```

### Options

```
      --bpf-filter string   bpf filter to apply, tcpdump -dd format
      --duration duration   duration of the capture
  -h, --help                help for pcap
  -i, --interface string    interface name to capture packets on (default "eth0")
  -o, --output string       if not set, decode packets to stdout; if set write raw pcap data to a file, use '-' for stdout
      --promiscuous         put interface into promiscuous mode
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl](#talosctl)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos

## talosctl processes

List running processes

```
talosctl processes [flags]
```

### Options

```
  -h, --help          help for processes
  -s, --sort string   Column to sort output by. [rss|cpu] (default "rss")
  -w, --watch         Stream running processes
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl](#talosctl)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos

## talosctl read

Read a file on the machine

```
talosctl read <path> [flags]
```

### Options

```
  -h, --help   help for read
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl](#talosctl)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos

## talosctl reboot

Reboot a node

```
talosctl reboot [flags]
```

### Options

```
      --debug              debug operation from kernel logs. --wait is set to true when this flag is set
  -h, --help               help for reboot
  -m, --mode string        select the reboot mode: "default", "powercycle" (skips kexec) (default "default")
      --timeout duration   time to wait for the operation is complete if --debug or --wait is set (default 30m0s)
      --wait               wait for the operation to complete, tracking its progress. always set to true when --debug is set (default true)
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl](#talosctl)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos

## talosctl reset

Reset a node

```
talosctl reset [flags]
```

### Options

```
      --debug                                    debug operation from kernel logs. --wait is set to true when this flag is set
      --graceful                                 if true, attempt to cordon/drain node and leave etcd (if applicable) (default true)
  -h, --help                                     help for reset
      --insecure                                 reset using the insecure (encrypted with no auth) maintenance service
      --reboot                                   if true, reboot the node after resetting instead of shutting down
      --system-labels-to-wipe strings            if set, just wipe selected system disk partitions by label but keep other partitions intact
      --timeout duration                         time to wait for the operation is complete if --debug or --wait is set (default 30m0s)
      --user-disks-to-wipe strings               if set, wipes defined devices in the list
      --wait                                     wait for the operation to complete, tracking its progress. always set to true when --debug is set (default true)
      --wipe-mode all, system-disk, user-disks   disk reset mode (default all)
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl](#talosctl)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos

## talosctl restart

Restart a process

```
talosctl restart <id> [flags]
```

### Options

```
  -h, --help         help for restart
  -k, --kubernetes   use the k8s.io containerd namespace
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl](#talosctl)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos

## talosctl rollback

Rollback a node to the previous installation

```
talosctl rollback [flags]
```

### Options

```
  -h, --help   help for rollback
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl](#talosctl)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos

## talosctl rotate-ca

Rotate cluster CAs (Talos and Kubernetes APIs).

### Synopsis

The command can rotate both Talos and Kubernetes root CAs (for the API).
By default both CAs are rotated, but you can choose to rotate just one or another.
The command starts by generating new CAs, and gracefully applying it to the cluster.

For Kubernetes, the command only rotates the API server issuing CA, and other Kubernetes
PKI can be rotated by applying machine config changes to the controlplane nodes.

```
talosctl rotate-ca [flags]
```

### Options

```
      --control-plane-nodes strings   specify IPs of control plane nodes
      --dry-run                       dry-run mode (no changes to the cluster) (default true)
  -h, --help                          help for rotate-ca
      --init-node string              specify IPs of init node
      --k8s-endpoint string           use endpoint instead of kubeconfig default
      --kubernetes                    rotate Kubernetes API CA (default true)
  -o, --output talosconfig            path to the output new talosconfig (default "talosconfig")
      --talos                         rotate Talos API CA (default true)
      --with-docs                     patch all machine configs adding the documentation for each field (default true)
      --with-examples                 patch all machine configs with the commented examples (default true)
      --worker-nodes strings          specify IPs of worker nodes
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl](#talosctl)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos

## talosctl service

Retrieve the state of a service (or all services), control service state

### Synopsis

Service control command. If run without arguments, lists all the services and their state.
If service ID is specified, default action 'status' is executed which shows status of a single list service.
With actions 'start', 'stop', 'restart', service state is updated respectively.

```
talosctl service [<id> [start|stop|restart|status]] [flags]
```

### Options

```
  -h, --help   help for service
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl](#talosctl)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos

## talosctl shutdown

Shutdown a node

```
talosctl shutdown [flags]
```

### Options

```
      --debug              debug operation from kernel logs. --wait is set to true when this flag is set
      --force              if true, force a node to shutdown without a cordon/drain
  -h, --help               help for shutdown
      --timeout duration   time to wait for the operation is complete if --debug or --wait is set (default 30m0s)
      --wait               wait for the operation to complete, tracking its progress. always set to true when --debug is set (default true)
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl](#talosctl)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos

## talosctl stats

Get container stats

```
talosctl stats [flags]
```

### Options

```
  -h, --help         help for stats
  -k, --kubernetes   use the k8s.io containerd namespace
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl](#talosctl)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos

## talosctl support

Dump debug information about the cluster

### Synopsis

Generated bundle contains the following debug information:

- For each node:

	- Kernel logs.
	- All Talos internal services logs.
	- All kube-system pods logs.
	- Talos COSI resources without secrets.
	- COSI runtime state graph.
	- Processes snapshot.
	- IO pressure snapshot.
	- Mounts list.
	- PCI devices info.
	- Talos version.

- For the cluster:

	- Kubernetes nodes and kube-system pods manifests.


```
talosctl support [flags]
```

### Options

```
  -h, --help              help for support
  -w, --num-workers int   number of workers per node (default 1)
  -O, --output string     output file to write support archive to
  -v, --verbose           verbose output
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl](#talosctl)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos

## talosctl time

Gets current server time

```
talosctl time [--check server] [flags]
```

### Options

```
  -c, --check string   checks server time against specified ntp server
  -h, --help           help for time
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl](#talosctl)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos

## talosctl upgrade

Upgrade Talos on the target node

```
talosctl upgrade [flags]
```

### Options

```
      --debug                debug operation from kernel logs. --wait is set to true when this flag is set
  -f, --force                force the upgrade (skip checks on etcd health and members, might lead to data loss)
  -h, --help                 help for upgrade
  -i, --image string         the container image to use for performing the install (default "ghcr.io/siderolabs/installer:v1.11.0-alpha.1")
      --insecure             upgrade using the insecure (encrypted with no auth) maintenance service
  -m, --reboot-mode string   select the reboot mode during upgrade. Mode "powercycle" bypasses kexec. Valid values are: ["default" "powercycle"]. (default "default")
  -s, --stage                stage the upgrade to perform it after a reboot
      --timeout duration     time to wait for the operation is complete if --debug or --wait is set (default 30m0s)
      --wait                 wait for the operation to complete, tracking its progress. always set to true when --debug is set (default true)
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl](#talosctl)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos

## talosctl upgrade-k8s

Upgrade Kubernetes control plane in the Talos cluster.

### Synopsis

Command runs upgrade of Kubernetes control plane components between specified versions.

```
talosctl upgrade-k8s [flags]
```

### Options

```
      --apiserver-image string            kube-apiserver image to use (default "registry.k8s.io/kube-apiserver")
      --controller-manager-image string   kube-controller-manager image to use (default "registry.k8s.io/kube-controller-manager")
      --dry-run                           skip the actual upgrade and show the upgrade plan instead
      --endpoint string                   the cluster control plane endpoint
      --from string                       the Kubernetes control plane version to upgrade from
  -h, --help                              help for upgrade-k8s
      --kubelet-image string              kubelet image to use (default "ghcr.io/siderolabs/kubelet")
      --pre-pull-images                   pre-pull images before upgrade (default true)
      --proxy-image string                kube-proxy image to use (default "registry.k8s.io/kube-proxy")
      --scheduler-image string            kube-scheduler image to use (default "registry.k8s.io/kube-scheduler")
      --to string                         the Kubernetes control plane version to upgrade to (default "1.33.1")
      --upgrade-kubelet                   upgrade kubelet service (default true)
      --with-docs                         patch all machine configs adding the documentation for each field (default true)
      --with-examples                     patch all machine configs with the commented examples (default true)
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl](#talosctl)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos

## talosctl usage

Retrieve a disk usage

```
talosctl usage [path1] [path2] ... [pathN] [flags]
```

### Options

```
  -a, --all             write counts for all files, not just directories
  -d, --depth int32     maximum recursion depth
  -h, --help            help for usage
  -H, --humanize        humanize size and time in the output
  -t, --threshold int   threshold exclude entries smaller than SIZE if positive, or entries greater than SIZE if negative
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl](#talosctl)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos

## talosctl validate

Validate config

```
talosctl validate [flags]
```

### Options

```
  -c, --config string   the path of the config file
  -h, --help            help for validate
  -m, --mode string     the mode to validate the config for (valid values are metal, cloud, and container)
      --strict          treat validation warnings as errors
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl](#talosctl)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos

## talosctl version

Prints the version

```
talosctl version [flags]
```

### Options

```
      --client     Print client version only
  -h, --help       help for version
  -i, --insecure   use Talos maintenance mode API
      --short      Print the short version
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl](#talosctl)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos

## talosctl wipe disk

Wipe a block device (disk or partition) which is not used as a volume

### Synopsis

Wipe a block device (disk or partition) which is not used as a volume.

Use device names as arguments, for example: vda or sda5.

```
talosctl wipe disk <device names>... [flags]
```

### Options

```
      --drop-partition   drop partition after wipe (if applicable)
  -h, --help             help for disk
      --method string    wipe method to use [FAST ZEROES] (default "FAST")
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl wipe](#talosctl-wipe)	 - Wipe block device or volumes

## talosctl wipe

Wipe block device or volumes

### Options

```
  -h, --help   help for wipe
```

### Options inherited from parent commands

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl](#talosctl)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos
* [talosctl wipe disk](#talosctl-wipe-disk)	 - Wipe a block device (disk or partition) which is not used as a volume

## talosctl

A CLI for out-of-band management of Kubernetes nodes created by Talos

### Options

```
      --cluster string       Cluster to connect to if a proxy endpoint is used.
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -h, --help                 help for talosctl
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file. Defaults to 'TALOSCONFIG' env variable if set, otherwise '$HOME/.talos/config' and '/var/run/secrets/talos.dev/config' in order.
```

### SEE ALSO

* [talosctl apply-config](#talosctl-apply-config)	 - Apply a new configuration to a node
* [talosctl bootstrap](#talosctl-bootstrap)	 - Bootstrap the etcd cluster on the specified node.
* [talosctl cgroups](#talosctl-cgroups)	 - Retrieve cgroups usage information
* [talosctl cluster](#talosctl-cluster)	 - A collection of commands for managing local docker-based or QEMU-based clusters
* [talosctl completion](#talosctl-completion)	 - Output shell completion code for the specified shell (bash, fish or zsh)
* [talosctl config](#talosctl-config)	 - Manage the client configuration file (talosconfig)
* [talosctl conformance](#talosctl-conformance)	 - Run conformance tests
* [talosctl containers](#talosctl-containers)	 - List containers
* [talosctl copy](#talosctl-copy)	 - Copy data out from the node
* [talosctl dashboard](#talosctl-dashboard)	 - Cluster dashboard with node overview, logs and real-time metrics
* [talosctl dmesg](#talosctl-dmesg)	 - Retrieve kernel logs
* [talosctl edit](#talosctl-edit)	 - Edit a resource from the default editor.
* [talosctl etcd](#talosctl-etcd)	 - Manage etcd
* [talosctl events](#talosctl-events)	 - Stream runtime events
* [talosctl gen](#talosctl-gen)	 - Generate CAs, certificates, and private keys
* [talosctl get](#talosctl-get)	 - Get a specific resource or list of resources (use 'talosctl get rd' to see all available resource types).
* [talosctl health](#talosctl-health)	 - Check cluster health
* [talosctl image](#talosctl-image)	 - Manage CRI container images
* [talosctl inject](#talosctl-inject)	 - Inject Talos API resources into Kubernetes manifests
* [talosctl inspect](#talosctl-inspect)	 - Inspect internals of Talos
* [talosctl kubeconfig](#talosctl-kubeconfig)	 - Download the admin kubeconfig from the node
* [talosctl list](#talosctl-list)	 - Retrieve a directory listing
* [talosctl logs](#talosctl-logs)	 - Retrieve logs for a service
* [talosctl machineconfig](#talosctl-machineconfig)	 - Machine config related commands
* [talosctl memory](#talosctl-memory)	 - Show memory usage
* [talosctl meta](#talosctl-meta)	 - Write and delete keys in the META partition
* [talosctl mounts](#talosctl-mounts)	 - List mounts
* [talosctl netstat](#talosctl-netstat)	 - Show network connections and sockets
* [talosctl patch](#talosctl-patch)	 - Update field(s) of a resource using a JSON patch.
* [talosctl pcap](#talosctl-pcap)	 - Capture the network packets from the node.
* [talosctl processes](#talosctl-processes)	 - List running processes
* [talosctl read](#talosctl-read)	 - Read a file on the machine
* [talosctl reboot](#talosctl-reboot)	 - Reboot a node
* [talosctl reset](#talosctl-reset)	 - Reset a node
* [talosctl restart](#talosctl-restart)	 - Restart a process
* [talosctl rollback](#talosctl-rollback)	 - Rollback a node to the previous installation
* [talosctl rotate-ca](#talosctl-rotate-ca)	 - Rotate cluster CAs (Talos and Kubernetes APIs).
* [talosctl service](#talosctl-service)	 - Retrieve the state of a service (or all services), control service state
* [talosctl shutdown](#talosctl-shutdown)	 - Shutdown a node
* [talosctl stats](#talosctl-stats)	 - Get container stats
* [talosctl support](#talosctl-support)	 - Dump debug information about the cluster
* [talosctl time](#talosctl-time)	 - Gets current server time
* [talosctl upgrade](#talosctl-upgrade)	 - Upgrade Talos on the target node
* [talosctl upgrade-k8s](#talosctl-upgrade-k8s)	 - Upgrade Kubernetes control plane in the Talos cluster.
* [talosctl usage](#talosctl-usage)	 - Retrieve a disk usage
* [talosctl validate](#talosctl-validate)	 - Validate config
* [talosctl version](#talosctl-version)	 - Prints the version
* [talosctl wipe](#talosctl-wipe)	 - Wipe block device or volumes

