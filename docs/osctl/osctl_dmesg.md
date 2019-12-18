<!-- markdownlint-disable -->
## osctl dmesg

Retrieve kernel logs

### Synopsis

Retrieve kernel logs

```
osctl dmesg [flags]
```

### Options

```
  -f, --follow   specify if the kernel log should be streamed
  -h, --help     help for dmesg
      --tail     specify if only new messages should be sent (makes sense only when combined with --follow)
```

### Options inherited from parent commands

```
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file (default "/root/.talos/config")
```

### SEE ALSO

* [osctl](osctl.md)	 - A CLI for out-of-band management of Kubernetes nodes created by Talos

