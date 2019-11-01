<!-- markdownlint-disable -->
## osctl

A CLI for out-of-band management of Kubernetes nodes created by Talos

### Synopsis

A CLI for out-of-band management of Kubernetes nodes created by Talos

### Options

```
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -h, --help                 help for osctl
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file (default "/root/.talos/config")
```

### SEE ALSO

* [osctl cluster](osctl_cluster.md)	 - A collection of commands for managing local docker-based clusters
* [osctl config](osctl_config.md)	 - Manage the client configuration
* [osctl containers](osctl_containers.md)	 - List containers
* [osctl copy](osctl_copy.md)	 - Copy data out from the node
* [osctl dmesg](osctl_dmesg.md)	 - Retrieve kernel logs
* [osctl gen](osctl_gen.md)	 - Generate CAs, certificates, and private keys
* [osctl interfaces](osctl_interfaces.md)	 - List network interfaces
* [osctl kubeconfig](osctl_kubeconfig.md)	 - Download the admin kubeconfig from the node
* [osctl list](osctl_list.md)	 - Retrieve a directory listing
* [osctl logs](osctl_logs.md)	 - Retrieve logs for a process or container
* [osctl memory](osctl_memory.md)	 - Show memory usage
* [osctl mounts](osctl_mounts.md)	 - List mounts
* [osctl processes](osctl_processes.md)	 - List running processes
* [osctl read](osctl_read.md)	 - Read a file on the machine
* [osctl reboot](osctl_reboot.md)	 - Reboot a node
* [osctl reset](osctl_reset.md)	 - Reset a node
* [osctl restart](osctl_restart.md)	 - Restart a process
* [osctl routes](osctl_routes.md)	 - List network routes
* [osctl service](osctl_service.md)	 - Retrieve the state of a service (or all services), control service state
* [osctl shutdown](osctl_shutdown.md)	 - Shutdown a node
* [osctl stats](osctl_stats.md)	 - Get processes stats
* [osctl time](osctl_time.md)	 - Gets current server time
* [osctl upgrade](osctl_upgrade.md)	 - Upgrade Talos on the target node
* [osctl validate](osctl_validate.md)	 - Validate config
* [osctl version](osctl_version.md)	 - Prints the version

