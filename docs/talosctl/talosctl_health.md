<!-- markdownlint-disable -->
## talosctl health

Check cluster health

### Synopsis

Check cluster health

```
talosctl health [flags]
```

### Options

```
      --control-plane-nodes strings   specify IPs of control plane nodes
  -h, --help                          help for health
      --init-node string              specify IPs of init node
      --k8s-endpoint string           use endpoint instead of kubeconfig default
      --run-e2e                       run Kubernetes e2e test
      --server                        run server-side check (default true)
      --wait-timeout duration         timeout to wait for the cluster to be ready (default 20m0s)
      --worker-nodes strings          specify IPs of worker nodes
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

