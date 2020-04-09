<!-- markdownlint-disable -->
## talosctl crashdump

Dump debug information about the cluster

### Synopsis

Dump debug information about the cluster

```
talosctl crashdump [flags]
```

### Options

```
      --control-plane-nodes strings   specify IPs of control plane nodes
  -h, --help                          help for crashdump
      --init-node string              specify IPs of init node
      --worker-nodes strings          specify IPs of worker nodes
```

### Options inherited from parent commands

```
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file (default "/home/user/.talos/config")
```

### SEE ALSO

* [talosctl](talosctl.md)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos

