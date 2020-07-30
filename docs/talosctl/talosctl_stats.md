<!-- markdownlint-disable -->
## talosctl stats

Get container stats

### Synopsis

Get container stats

```
talosctl stats [flags]
```

### Options

```
  -h, --help         help for stats
  -k, --kubernetes   use the k8s.io containerd namespace
  -c, --use-cri      use the CRI driver
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

