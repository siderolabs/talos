<!-- markdownlint-disable -->
## talosctl usage

Retrieve a disk usage

### Synopsis

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
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file (default "/home/user/.talos/config")
```

### SEE ALSO

* [talosctl](talosctl.md)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos

