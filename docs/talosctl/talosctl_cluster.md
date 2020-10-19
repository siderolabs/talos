<!-- markdownlint-disable -->
## talosctl cluster

A collection of commands for managing local docker-based or firecracker-based clusters

### Options

```
  -h, --help                 help for cluster
      --name string          the name of the cluster (default "talos-default")
      --provisioner string   Talos cluster provisioner to use (default "docker")
      --state string         directory path to store cluster state (default "/home/user/.talos/clusters")
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
* [talosctl cluster create](talosctl_cluster_create.md)	 - Creates a local docker-based or QEMU-based kubernetes cluster
* [talosctl cluster destroy](talosctl_cluster_destroy.md)	 - Destroys a local docker-based or firecracker-based kubernetes cluster
* [talosctl cluster show](talosctl_cluster_show.md)	 - Shows info about a local provisioned kubernetes cluster

