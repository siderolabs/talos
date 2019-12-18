<!-- markdownlint-disable -->
## osctl cluster

A collection of commands for managing local docker-based clusters

### Synopsis

A collection of commands for managing local docker-based clusters

### Options

```
  -h, --help          help for cluster
      --name string   the name of the cluster (default "talos-default")
```

### Options inherited from parent commands

```
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file (default "/home/user/.talos/config")
```

### SEE ALSO

* [osctl](osctl.md)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos
* [osctl cluster create](osctl_cluster_create.md)	 - Creates a local docker-based kubernetes cluster
* [osctl cluster destroy](osctl_cluster_destroy.md)	 - Destroys a local docker-based kubernetes cluster

