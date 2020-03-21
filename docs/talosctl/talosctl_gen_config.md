<!-- markdownlint-disable -->
## talosctl gen config

Generates a set of configuration files for Talos cluster

### Synopsis

Generates a set of configuration files for Talos cluster

```
talosctl gen config <cluster name> https://<load balancer IP or DNS name> [flags]
```

### Options

```
      --additional-sans strings     additional Subject-Alt-Names for the APIServer certificate
      --dns-domain string           the dns domain to use for cluster (default "cluster.local")
  -h, --help                        help for config
      --install-disk string         the disk to install to (default "/dev/sda")
      --install-image string        the image used to perform an installation (default "docker.io/autonomy/installer:latest")
      --kubernetes-version string   desired kubernetes version to run (default "1.17.1")
  -o, --output-dir string           destination to output generated files
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

