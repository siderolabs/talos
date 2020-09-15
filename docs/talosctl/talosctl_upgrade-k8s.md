<!-- markdownlint-disable -->
## talosctl upgrade-k8s

Upgrade Kubernetes control plane in the Talos cluster.

### Synopsis

Command runs upgrade of Kubernetes control plane components between specified versions. Pod-checkpointer is handled in a special way to speed up kube-apisever upgrades.

```
talosctl upgrade-k8s [flags]
```

### Options

```
      --arch string   the cluster architecture (default "amd64")
      --from string   the Kubernetes control plane version to upgrade from
  -h, --help          help for upgrade-k8s
      --to string     the Kubernetes control plane version to upgrade to (default "1.19.1")
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

