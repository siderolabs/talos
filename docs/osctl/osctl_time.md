<!-- markdownlint-disable -->
## osctl time

Gets current server time

### Synopsis

Gets current server time

```
osctl time [--check server] [flags]
```

### Options

```
  -c, --check string   checks server time against specified ntp server (default "pool.ntp.org")
  -h, --help           help for time
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

