<!-- markdownlint-disable -->
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
      --additional-sans strings     additional Subject-Alt-Names for the APIServer certificate
      --arch string                 the architecture of the cluster (default "amd64")
      --dns-domain string           the dns domain to use for cluster (default "cluster.local")
  -h, --help                        help for config
      --install-disk string         the disk to install to (default "/dev/sda")
      --install-image string        the image used to perform an installation (default "ghcr.io/talos-systems/installer:latest")
      --kubernetes-version string   desired kubernetes version to run (default "1.19.1")
  -o, --output-dir string           destination to output generated files
  -p, --persist                     the desired persist value for configs (default true)
      --registry-mirror strings     list of registry mirrors to use in format: <registry host>=<mirror URL>
      --version string              the desired machine config version to generate (default "v1alpha1")
```

### Options inherited from parent commands

```
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file (default "/home/user/.talos/config")
```

### SEE ALSO

* [talosctl gen](talosctl_gen.md)	 - Generate CAs, certificates, and private keys

