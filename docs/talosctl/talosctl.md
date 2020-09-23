<!-- markdownlint-disable -->
## talosctl

A CLI for out-of-band management of Kubernetes nodes created by Talos

### Synopsis

A CLI for out-of-band management of Kubernetes nodes created by Talos

### Options

```
      --context string       Context to be used in command
  -e, --endpoints strings    override default endpoints in Talos configuration
  -h, --help                 help for talosctl
  -n, --nodes strings        target the specified nodes
      --talosconfig string   The path to the Talos configuration file (default "/home/user/.talos/config")
```

### SEE ALSO

* [talosctl apply-config](talosctl_apply-config.md)	 - Apply a new configuration to a node
* [talosctl bootstrap](talosctl_bootstrap.md)	 - Bootstrap the cluster
* [talosctl cluster](talosctl_cluster.md)	 - A collection of commands for managing local docker-based or firecracker-based clusters
* [talosctl completion](talosctl_completion.md)	 - Output shell completion code for the specified shell (bash or zsh)
* [talosctl config](talosctl_config.md)	 - Manage the client configuration
* [talosctl containers](talosctl_containers.md)	 - List containers
* [talosctl copy](talosctl_copy.md)	 - Copy data out from the node
* [talosctl crashdump](talosctl_crashdump.md)	 - Dump debug information about the cluster
* [talosctl dashboard](talosctl_dashboard.md)	 - Cluster dashboard with real-time metrics
* [talosctl dmesg](talosctl_dmesg.md)	 - Retrieve kernel logs
* [talosctl events](talosctl_events.md)	 - Stream runtime events
* [talosctl gen](talosctl_gen.md)	 - Generate CAs, certificates, and private keys
* [talosctl health](talosctl_health.md)	 - Check cluster health
* [talosctl images](talosctl_images.md)	 - List the default images used by Talos
* [talosctl interfaces](talosctl_interfaces.md)	 - List network interfaces
* [talosctl kubeconfig](talosctl_kubeconfig.md)	 - Download the admin kubeconfig from the node
* [talosctl list](talosctl_list.md)	 - Retrieve a directory listing
* [talosctl logs](talosctl_logs.md)	 - Retrieve logs for a service
* [talosctl memory](talosctl_memory.md)	 - Show memory usage
* [talosctl mounts](talosctl_mounts.md)	 - List mounts
* [talosctl processes](talosctl_processes.md)	 - List running processes
* [talosctl read](talosctl_read.md)	 - Read a file on the machine
* [talosctl reboot](talosctl_reboot.md)	 - Reboot a node
* [talosctl recover](talosctl_recover.md)	 - Recover a control plane
* [talosctl reset](talosctl_reset.md)	 - Reset a node
* [talosctl restart](talosctl_restart.md)	 - Restart a process
* [talosctl rollback](talosctl_rollback.md)	 - Rollback a node to the previous installation
* [talosctl routes](talosctl_routes.md)	 - List network routes
* [talosctl service](talosctl_service.md)	 - Retrieve the state of a service (or all services), control service state
* [talosctl shutdown](talosctl_shutdown.md)	 - Shutdown a node
* [talosctl stats](talosctl_stats.md)	 - Get container stats
* [talosctl time](talosctl_time.md)	 - Gets current server time
* [talosctl upgrade](talosctl_upgrade.md)	 - Upgrade Talos on the target node
* [talosctl upgrade-k8s](talosctl_upgrade-k8s.md)	 - Upgrade Kubernetes control plane in the Talos cluster.
* [talosctl validate](talosctl_validate.md)	 - Validate config
* [talosctl version](talosctl_version.md)	 - Prints the version

