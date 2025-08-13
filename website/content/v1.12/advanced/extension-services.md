---
title: "Extension Services"
description: "Use extension services in Talos Linux."
aliases:
  - ../learn-more/extension-services
---

Talos provides a way to run additional system services early in the Talos boot process.
Extension services should be included into the Talos root filesystem (e.g. using [system extensions]({{< relref "../talos-guides/configuration/system-extensions" >}})).
Extension services run as privileged containers with ephemeral root filesystem located in the Talos root filesystem.

Extension services can be used to use extend core features of Talos in a way that is not possible via [static pods]({{< relref "../advanced/static-pods" >}}) or
Kubernetes DaemonSets.

Potential extension services use-cases:

* storage: Open iSCSI, software RAID, etc.
* networking: BGP FRR, etc.
* platform integration: VMWare open VM tools, etc.

## Configuration

Talos on boot scans directory `/usr/local/etc/containers` for `*.yaml` files describing the extension services to run.
Format of the extension service config:

```yaml
name: hello-world
container:
  entrypoint: ./hello-world
  environment:
    - XDG_RUNTIME_DIR=/run
  args:
     - -f
  mounts:
     - # OCI Mount Spec
depends:
   - configuration: true
   - service: cri
   - path: /run/machined/machined.sock
   - network:
       - addresses
       - connectivity
       - hostname
       - etcfiles
   - time: true
restart: never|always|untilSuccess
logToConsole: true|false
```

### `name`

Field `name` sets the service name, valid names are `[a-z0-9-_]+`.
The service container root filesystem path is derived from the `name`: `/usr/local/lib/containers/<name>`.
The extension service will be registered as a Talos service under an `ext-<name>` identifier.

### `container`

* `entrypoint` defines the container entrypoint relative to the container root filesystem (`/usr/local/lib/containers/<name>`)
* `environmentFile` (**deprecated**) defines the path to a file containing environment variables, the service waits for the file to
  exist before starting.
  Use `ExtensionServiceConfig` instead.
* `environment` defines the container environment variables.
* `args` defines the additional arguments to pass to the entrypoint
* `mounts` defines the volumes to be mounted into the container root

#### `container.mounts`

The section `mounts` uses the standard OCI spec:

```yaml
- source: /var/log/audit
  destination: /var/log/audit
  type: bind
  options:
    - rshared
    - bind
    - ro
```

All requested directories will be mounted into the extension service container mount namespace.
If the `source` directory doesn't exist in the host filesystem, it will be created (only for writable paths in the Talos root filesystem).

#### `container.security`

The section `security` follows this example:

```yaml
maskedPaths:
  - "/should/be/masked"
readonlyPaths:
  - "/path/that/should/be/readonly"
  - "/another/readonly/path"
writeableRootfs: true
writeableSysfs: true
rootfsPropagation: shared
```

> * The rootfs is readonly by default unless `writeableRootfs: true` is set.
> * The sysfs is readonly by default unless `writeableSysfs: true` is set.
> * Masked paths if not set defaults to [containerd defaults](https://github.com/containerd/containerd/tree/main/oci/spec.go).
Masked paths will be mounted to `/dev/null`.
To set empty masked paths use:
>
> ```yaml
> container:
>   security:
>     maskedPaths: []
> ```
>
> * Read Only paths if not set defaults to [containerd defaults](https://github.com/containerd/containerd/tree/main/oci/spec.go).
Read-only paths will be mounted to `/dev/null`.
To set empty read only paths use:
>
> ```yaml
> container:
>   security:
>     readonlyPaths: []
> ```
>
> * Rootfs propagation is not set by default (container mounts are private).

### `depends`

The `depends` section describes extension service start dependencies: the service will not be started until all dependencies are met.

Available dependencies:

* `service: <name>`: wait for the service `<name>` to be running and healthy
* `path: <path>`: wait for the `<path>` to exist
* `network: [addresses, connectivity, hostname, etcfiles]`: wait for the specified network readiness checks to succeed
* `time: true`: wait for the NTP time sync
* `configuration: true`: wait for `ExtensionServiceConfig` resource with a name matching the extension name to be available.
  The mounts specified in the `ExtensionServiceConfig` will be added as extra mounts to the extension service.

### `restart`

Field `restart` defines the service restart policy, it allows to either configure an always running service or a one-shot service:

* `always`: restart service always
* `never`: start service only once and never restart
* `untilSuccess`: restart failing service, stop restarting on successful run

### `logToConsole`

Field `logToConsole` defines whether the service logs should also be written to the console, i.e., to kernel log buffer (or to the container logs in container mode).

This feature is particularly useful for debugging extensions that operate in maintenance mode or early in the boot process when service logs cannot be accessed yet.

## Example

Example layout of the Talos root filesystem contents for the extension service:

```text
/
└── usr
    └── local
        ├── etc
        │   └── containers
        │       └── hello-world.yaml
        └── lib
            └── containers
                └── hello-world
                    ├── hello
                    └── config.ini
```

Talos discovers the extension service configuration in `/usr/local/etc/containers/hello-world.yaml`:

```yaml
name: hello-world
container:
  entrypoint: ./hello
  args:
    - --config
    - config.ini
depends:
  - network:
    - addresses
restart: always
```

Talos starts the container for the extension service with container root filesystem at `/usr/local/lib/containers/hello-world`:

```text
/
├── hello
└── config.ini
```

Extension service is registered as `ext-hello-world` in `talosctl services`:

```shell
$ talosctl service ext-hello-world
NODE     172.20.0.5
ID       ext-hello-world
STATE    Running
HEALTH   ?
EVENTS   [Running]: Started task ext-hello-world (PID 1100) for container ext-hello-world (2m47s ago)
         [Preparing]: Creating service runner (2m47s ago)
         [Preparing]: Running pre state (2m47s ago)
         [Waiting]: Waiting for service "containerd" to be "up" (2m48s ago)
         [Waiting]: Waiting for service "containerd" to be "up", network (2m49s ago)
```

An extension service can be started, restarted and stopped using `talosctl service ext-hello-world start|restart|stop`.
Use `talosctl logs ext-hello-world` to get the logs of the service.

Complete example of the extension service can be found in the [extensions repository](https://github.com/talos-systems/extensions/tree/main/examples/hello-world-service).
