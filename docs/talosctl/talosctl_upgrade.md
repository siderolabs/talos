<!-- markdownlint-disable -->
## talosctl upgrade

Upgrade Talos on the target node

### Synopsis

Upgrade Talos on the target node

```
talosctl upgrade [flags]
```

### Options

```
  -h, --help           help for upgrade
  -i, --image string   the container image to use for performing the install
  -p, --preserve       preserve data
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

