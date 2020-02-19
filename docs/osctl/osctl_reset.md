<!-- markdownlint-disable -->
## osctl reset

Reset a node

### Synopsis

Reset a node

```
osctl reset [flags]
```

### Options

```
      --graceful   if true, attempt to cordon/drain node and leave etcd (if applicable) (default true)
  -h, --help       help for reset
      --reboot     if true, reboot the node after resetting instead of shutting down
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

