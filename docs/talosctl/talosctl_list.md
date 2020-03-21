<!-- markdownlint-disable -->
## talosctl list

Retrieve a directory listing

### Synopsis

Retrieve a directory listing

```
talosctl list [path] [flags]
```

### Options

```
  -d, --depth int32   maximum recursion depth
  -h, --help          help for list
  -H, --humanize      humanize size and time in the output
  -l, --long          display additional file details
  -r, --recurse       recurse into subdirectories
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

