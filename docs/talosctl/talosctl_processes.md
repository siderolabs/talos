<!-- markdownlint-disable -->
## talosctl processes

List running processes

```
talosctl processes [flags]
```

### Options

```
  -h, --help          help for processes
  -s, --sort string   Column to sort output by. [rss|cpu] (default "rss")
  -w, --watch         Stream running processes
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

