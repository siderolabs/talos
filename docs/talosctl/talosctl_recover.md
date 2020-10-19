<!-- markdownlint-disable -->
## talosctl recover

Recover a control plane

```
talosctl recover [flags]
```

### Options

```
  -h, --help            help for recover
  -s, --source string   The data source for restoring the control plane manifests from (valid options are "apiserver" and "etcd") (default "apiserver")
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

