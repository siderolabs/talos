<!-- markdownlint-disable -->
## talosctl events

Stream runtime events

### Synopsis

Stream runtime events

```
talosctl events [flags]
```

### Options

```
      --duration duration   show events for the past duration interval (one second resolution, default is to show no history)
  -h, --help                help for events
      --since string        show events after the specified event ID (default is to show no history)
      --tail int32          show specified number of past events (use -1 to show full history, default is to show no history)
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

