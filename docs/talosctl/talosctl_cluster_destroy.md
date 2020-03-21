<!-- markdownlint-disable -->
## talosctl cluster destroy

Destroys a local docker-based or firecracker-based kubernetes cluster

### Synopsis

Destroys a local docker-based or firecracker-based kubernetes cluster

```
talosctl cluster destroy [flags]
```

### Options

```
  -h, --help   help for destroy
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

