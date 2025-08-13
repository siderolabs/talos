---
title: "Developing Talos"
description: "Learn how to set up a development environment for local testing and hacking on Talos itself!"
aliases:
  - ../learn-more/developing-talos
---

This guide outlines steps and tricks to develop Talos operating systems and related components.
The guide assumes macOS or a Linux operating system on the development host.

## Prepare

Check out the [Talos repository](https://github.com/siderolabs/talos).

Try running `make help` to see available `make` commands.
You would need Docker and `buildx` installed on the host.

> Note: Usually it is better to install up to date Docker from Docker apt repositories, e.g. [Ubuntu instructions](https://docs.docker.com/engine/install/ubuntu/).
>
> If `buildx` plugin is not available with OS docker packages, it can be installed [as a plugin from GitHub releases](https://docs.docker.com/buildx/working-with-buildx/#install).

Set up a builder with access to the host network:

```bash
 docker buildx create --driver docker-container  --driver-opt network=host --name local1 --buildkitd-flags '--allow-insecure-entitlement security.insecure' --use
```

> Note: `network=host` allows buildx builder to access host network, so that it can push to a local container registry (see below).

Make sure the following steps work:

- `make talosctl`
- `make initramfs kernel`

Set up a local docker registry:

```bash
docker run -d -p 5005:5000 \
    --restart always \
    --name local registry:2
```

Try to build and push to local registry an installer image:

```bash
make installer-base IMAGE_REGISTRY=127.0.0.1:5005 PUSH=true
make imager IMAGE_REGISTRY=127.0.0.1:5005 PUSH=true INSTALLER_ARCH=targetarch
make installer IMAGE_REGISTRY=127.0.0.1:5005 PUSH=true
```

Record the image name output in the step above.

> Note: it is also possible to force a stable image tag by using `TAG` variable: `make installer-base IMAGE_REGISTRY=127.0.0.1:5005 TAG=v1.0.0-alpha.1 PUSH=true`.

## Running Talos cluster

Set up local caching docker registries (this speeds up Talos cluster boot a lot), script is in the Talos repo:

```bash
bash hack/start-registry-proxies.sh
```

Start your local cluster with:

```bash
sudo --preserve-env=HOME _out/talosctl-<YOUR FLAVOR> cluster create \
    --provisioner=qemu \
    --cidr=172.20.0.0/24 \
    --registry-mirror docker.io=http://172.20.0.1:5000 \
    --registry-mirror registry.k8s.io=http://172.20.0.1:5001  \
    --registry-mirror gcr.io=http://172.20.0.1:5003 \
    --registry-mirror ghcr.io=http://172.20.0.1:5004 \
    --registry-mirror 127.0.0.1:5005=http://172.20.0.1:5005 \
    --install-image=127.0.0.1:5005/siderolabs/installer:<RECORDED HASH from the build step> \
    --controlplanes 3 \
    --workers 2 \
    --with-bootloader=false
```

- `--provisioner` selects QEMU vs. default Docker
- custom `--cidr` to make QEMU cluster use different network than default Docker setup (optional)
- `--registry-mirror` uses the caching proxies set up above to speed up boot time a lot, last one adds your local registry (installer image was pushed to it)
- `--install-image` is the image you built with `make installer` above
- `--controlplanes` & `--workers` configure cluster size, choose to match your resources; 3 controlplanes give you HA control plane; 1 controlplane is enough, never do 2 controlplanes
- `--with-bootloader=false` disables boot from disk (Talos will always boot from `_out/vmlinuz-<ARCH>` and `_out/initramfs-<ARCH>.xz`).
  This speeds up development cycle a lot - no need to rebuild installer and perform an install, rebooting is enough to get new code changes.

> Note: when configuration changes are introduced and the old installer doesn't validate the config, or the installation flow itself is being worked on  `--with-bootloader=false` should not be used
>
> `talosctl cluster create` derives Talos machine configuration version from the install image tag, so sometimes early in the development cycle (when new minor tag is not released yet), machine config version can be overridden with `--talos-version={{< version >}}`.

## Console Logs

Watching console logs is easy with `tail`:

```bash
tail -F ~/.talos/clusters/talos-default/talos-default-*.log
```

## Interacting with Talos

Once `talosctl cluster create` finishes successfully, `talosconfig` and `kubeconfig` will be set up automatically to point to your cluster.

Start playing with `talosctl`:

```bash
talosctl -n 172.20.0.2 version
talosctl -n 172.20.0.3,172.20.0.4 dashboard
talosctl -n 172.20.0.4 get members
```

Same with `kubectl`:

```bash
kubectl get nodes -o wide
```

You can deploy some Kubernetes workloads to the cluster.

You can edit machine config on the fly with `talosctl edit mc --immediate`, config patches can be applied via `--config-patch` flags, also many features have specific flags in `talosctl cluster create`.

## Quick Reboot

To reboot whole cluster quickly (e.g. to pick up a change made in the code):

```bash
for socket in ~/.talos/clusters/talos-default/talos-default-*.monitor; do echo "q" | sudo socat - unix-connect:$socket; done
```

Sending `q` to a single socket allows to reboot a single node.

> Note: This command performs immediate reboot (as if the machine was powered down and immediately powered back up), for normal Talos reboot use `talosctl reboot`.

## Development Cycle

Fast development cycle:

- bring up a cluster
- make code changes
- rebuild `initramfs` with `make initramfs`
- reboot a node to pick new `initramfs`
- verify code changes
- more code changes...

Some aspects of Talos development require to enable bootloader (when working on `installer` itself), in that case quick development cycle is no longer possible, and cluster should be destroyed and recreated each time.

## Running Integration Tests

If integration tests were changed (or when running them for the first time), first rebuild the integration test binary:

```bash
rm -f  _out/integration-test-linux-amd64; make _out/integration-test-linux-amd64
```

Running short tests against QEMU provisioned cluster:

```bash
_out/integration-test-linux-amd64 \
    -talos.provisioner=qemu \
    -test.v \
    -test.short \
    -talos.talosctlpath=$PWD/_out/talosctl-linux-amd64
```

Whole test suite can be run removing `-test.short` flag.

Specfic tests can be run with `-test.run=TestIntegration/api.ResetSuite`.

## Build Flavors

`make <something> WITH_RACE=1` enables Go race detector, Talos runs slower and uses more memory, but memory races are detected.

`make <something> WITH_DEBUG=1` enables Go profiling and other debug features, useful for local development.

`make initramfs WITH_DEBUG_SHELL=true` adds bash and minimal utilities for debugging purposes.
Combine with `--with-debug-shell` flag when creating cluster to obtain shell access.
This is uncommonly used as in this case the bash shell will run in place of machined.

## Destroying Cluster

```bash
sudo --preserve-env=HOME ../talos/_out/talosctl-linux-amd64 cluster destroy --provisioner=qemu
```

This command stops QEMU and helper processes, tears down bridged network on the host, and cleans up
cluster state in `~/.talos/clusters`.

> Note: if the host machine is rebooted, QEMU instances and helpers processes won't be started back.
> In that case it's required to clean up files in `~/.talos/clusters/<cluster-name>` directory manually.

## Optional

Set up cross-build environment with:

```bash
docker run --rm --privileged multiarch/qemu-user-static --reset -p yes
```

> Note: the static qemu binaries which come with Ubuntu 21.10 seem to be broken.

## Unit tests

Unit tests can be run in buildx with `make unit-tests`, on Ubuntu systems some tests using `loop` devices will fail because Ubuntu uses low-index `loop` devices for snaps.

Most of the unit-tests can be run standalone as well, with regular `go test`, or using IDE integration:

```bash
go test -v ./internal/pkg/circular/
```

This provides much faster feedback loop, but some tests require either elevated privileges (running as `root`) or additional binaries available only in Talos `rootfs` (containerd tests).

Running tests as root can be done with `-exec` flag to `go test`, but this is risky, as test code has root access and can potentially make undesired changes:

```bash
go test -exec sudo  -v ./internal/app/machined/pkg/controllers/network/...
```

## Go Profiling

Build `initramfs` with debug enabled: `make initramfs WITH_DEBUG=1`.

Launch Talos cluster with bootloader disabled, and use `go tool pprof` to capture the profile and show the output in your browser:

```bash
go tool pprof http://172.20.0.2:9982/debug/pprof/heap
```

The IP address `172.20.0.2` is the address of the Talos node, and port `:9982` depends on the Go application to profile:

- 9981: `apid`
- 9982: `machined`
- 9983: `trustd`

## Testing Air-gapped Environments

There is a hidden `talosctl debug air-gapped` command which launches two components:

- HTTP proxy capable of proxying HTTP and HTTPS requests
- HTTPS server with a self-signed certificate

The command also writes down Talos machine configuration patch to enable the HTTP proxy and add a self-signed certificate
to the list of trusted certificates:

```shell
$ talosctl debug air-gapped --advertised-address 172.20.0.1
2022/08/04 16:43:14 writing config patch to air-gapped-patch.yaml
2022/08/04 16:43:14 starting HTTP proxy on :8002
2022/08/04 16:43:14 starting HTTPS server with self-signed cert on :8001
```

The `--advertised-address` should match the bridge IP of the Talos node.

Generated machine configuration patch looks like:

```yaml
machine:
    env:
        http_proxy: http://172.20.0.1:8002
        https_proxy: http://172.20.0.1:8002
        no_proxy: 172.20.0.1/24
cluster:
    extraManifests:
        - https://172.20.0.1:8001/debug.yaml
---
apiVersion: v1alpha1
kind: TrustedRootsConfig
name: air-gapped-ca
certificates: |
  -----BEGIN CERTIFICATE-----
  MIIBiTCCAS+gAwIBAgIBATAKBggqhkjOPQQDAjAUMRIwEAYDVQQKEwlUZXN0IE9u
  bHkwHhcNMjUwMTE1MTE1OTI3WhcNMjUwMTE2MTE1OTI3WjAUMRIwEAYDVQQKEwlU
  ZXN0IE9ubHkwWTATBgcqhkjOPQIBBggqhkjOPQMBBwNCAAReznBeEcQFcB/y1yqI
  HQcP0IWBMvgwGTeaaTBM6rV+AjbnyxgCrXAnmJ0t45Eur27eW9J/1T5tzA6fe24f
  YyY9o3IwcDAOBgNVHQ8BAf8EBAMCBaAwHQYDVR0lBBYwFAYIKwYBBQUHAwEGCCsG
  AQUFBwMCMA8GA1UdEwEB/wQFMAMBAf8wHQYDVR0OBBYEFEGBbafXsyzxVhVqfjzy
  7aBmVvtaMA8GA1UdEQQIMAaHBKwUAAEwCgYIKoZIzj0EAwIDSAAwRQIhAPAFm6Lv
  1Bw+M55Z1SEDLyILJSS0En5F6n8Q9LyGGT4fAiBi+Fm3wSQcvgGPG9OfokFaXmGp
  Pa6c4ZrarKO8ZxWigA==
  -----END CERTIFICATE-----
```

The first section appends a self-signed certificate of the HTTPS server to the list of trusted certificates,
followed by the HTTP proxy setup (in-cluster traffic is excluded from the proxy).
The last section adds an extra Kubernetes manifest hosted on the HTTPS server.

The machine configuration patch can now be used to launch a test Talos cluster:

```shell
talosctl cluster create ... --config-patch @air-gapped-patch.yaml
```

The following lines should appear in the output of the `talosctl debug air-gapped` command:

- `CONNECT discovery.talos.dev:443`: the HTTP proxy is used to talk to the discovery service
- `http: TLS handshake error from 172.20.0.2:53512: remote error: tls: bad certificate`: an expected error on Talos side, as self-signed cert is not written yet to the file
- `GET /debug.yaml`: Talos successfully fetches the extra manifest successfully

There might be more output depending on the registry caches being used or not.

## Running Upgrade Integration Tests

Talos has a separate set of provision upgrade tests, which create a cluster on older versions of Talos, perform an upgrade,
and verify that the cluster is still functional.

Build the test binary:

```bash
rm -f  _out/integration-test-provision-linux-amd64; make _out/integration-test-provision-linux-amd64
```

Prepare the test artifacts for the upgrade test:

```bash
make release-artifacts
```

Build and push an installer image for the development version of Talos:

```bash
make installer-base IMAGE_REGISTRY=127.0.0.1:5005 PUSH=true
make imager IMAGE_REGISTRY=127.0.0.1:5005 PUSH=true
make installer IMAGE_REGISTRY=127.0.0.1:5005
```

Run the tests (the tests will create the cluster on the older version of Talos, perform an upgrade, and verify that the cluster is still functional):

```bash
sudo --preserve-env=HOME _out/integration-test-provision-linux-amd64 \
    -test.v \
    -talos.talosctlpath _out/talosctl-linux-amd64 \
    -talos.provision.target-installer-registry=127.0.0.1:5005 \
    -talos.provision.registry-mirror 127.0.0.1:5005=http://172.20.0.1:5005,docker.io=http://172.20.0.1:5000,registry.k8s.io=http://172.20.0.1:5001,quay.io=http://172.20.0.1:5002,gcr.io=http://172.20.0.1:5003,ghcr.io=http://172.20.0.1:5004 \
    -talos.provision.cidr 172.20.0.0/24
```

## SELinux policy debugging and development

Here are some tips about how Talos SELinux policy is built, which should mainly help developers troubleshoot denials and assess policy rules for security against different threats.

### Obtaining and processing denial logs

If SELinux has blocked some event from happening, it will log it to the audit log.
If the mode is permissive, the only implication of would be a denial message, so permissive mode is useful for prototyping the policy.
You can check the logs with:

`talosctl --nodes 172.20.0.2 logs auditd > audit.log`

The obtained logs can be processed with `audit2allow` to obtain a CIL code that would allow the denied event to happen, alongside an explanation of the denial.
For this we use SELinux userspace utilities, which can be ran in a container for cases you use a Linux system without SELinux or another OS.
Some of the useful commands are:

```bash
audit2why -p ./internal/pkg/selinux/policy/policy.33 -i audit.log
audit2allow -C -e -p ./internal/pkg/selinux/policy/policy.33 -i audit.log
```

However, please do not consider the output of `audit2allow` as a final modification for the policy.
It is a good starting point to understand the denial, but the generated code should be reviewed and correctly reformulated once confirmed to be needed and not caused by mislabeling.

### Iterating on the policy

`make generate` generates the compiled SELinux files.
However, if you want to iterate on the policy rapidly, you might want to consider only rebuilding the policy during the testing:

```bash
make local-selinux-generate DEST=./internal/pkg/selinux PLATFORM=linux/amd64 PROGRESS=plain
```

### Debugging locally with many denials happening

Sometimes, e.g. during a major refactor, the policy can be broken and many denials can happen.
This can cause the audit ring buffer to fill up, losing some messages.
These are some kernel cmdline parameters that redirect the audit logs to the console, which is saved to your development cluster directory:

`talos.auditd.disabled=1 audit=1 audit_backlog_limit=65535 debug=1 sysctl.kernel.printk_ratelimit=0 sysctl.kernel.printk_delay=0 sysctl.kernel.printk_ratelimit_burst=10000`

### SELinux policy structure

The SELinux policy is built using the CIL language.
The CIL files are located in `internal/pkg/selinux/policy/selinux` and are compiled into a binary format (e.g. `33` for the current kernel policy format version) using the `secilc` tool from Talos tools bundle.
The policy is embedded into the initramfs init and loaded early in the boot process.

For understanding and modifying the policy, [CIL language reference](https://github.com/SELinuxProject/selinux-notebook/blob/dfabf5f1bcdc72e440c1f7010e39ae3ce9f0c364/src/notebook-examples/selinux-policy/cil/CIL_Reference_Guide.pdf) is a recommended starting point to get familiar with the language.
[Object Classes and Permissions](https://github.com/SELinuxProject/selinux-notebook/blob/dfabf5f1bcdc72e440c1f7010e39ae3ce9f0c364/src/object_classes_permissions.md) is another helpful document, listing all SELinux entities and the meaning of all the permissions.

The policy directory contains the following main subdirectories:

- `immutable`: contains the preamble parts, mostly listing SELinux SIDs, classes, policy capabilities and roles, not expected to change frequently.
- `common`: abstractions and common rules, which are used by the other parts of the policy or by all objects of some kind.:
  - classmaps: contains class maps, which are a SELinux concept for easily configuring the same list of permissions on a list of classes.
  Our policy frequently uses `fs_classes` classmap for enabling a group of file operations on all types of files.
  - files: labels for common system files, stored on squashfs.
  Mostly used for generalized labels not related to a particular service.
  - network: rules that allow basically any network activity, as Talos does not currently use SELinux features like IPsec labeling for network security.
  - typeattributes: this file contains typeattributes, which are a SELinux concept for grouping types together to have the same rules applied to all of them.
  This file also contains macros used to assign objects into typeattributes.
  When such a macro exists its use is recommended over using the typeattribute directly, as it allows for grepping for the macro call.
  - processes: common rules, applied to all processes or typeattribute of processes.
  We only add rules that apply widely here, with more specific rules being added to the service policy files.
- `services`: policy files for each service.
These files contain the definitions and rules that are specific to the service, like allowing access to its configuration files or communicating over sockets.
Some specific parts not being a service in the Talos terms are:
  - `selinux` - selinuxfs rules protecting SELinux settings from modifications after the OS has started.
  - `system-containerd` - a containerd instance used for `apid` and similar services internal to Talos.
  - `system-containers` - `apid`, `trustd`, `etcd` and other system services, running in system containerd instance.

#### classmaps overview

- `fs_classes` - contains file classes and their permissions, used for file operations.
  - `rw` - all operations, except SELinux label management.
  - `ro` - read-only operations.
  - others - just a class permission applied to all supported file classes.
- `netlink_classes (full)` - full (except security labels) access to all netlink socket classes.
- `process_classes` - helpers to allow a wide range of process operations.
  - `full` - all operations, except ptrace (considered to be a rare requirement, so should be added specifically where needed).
  - `signal` - send any signal to the target process.

#### typeattributes overview

- Processes:
  - `service_p` - system services.
  - `system_container_p` - containerized system services.
  - `pod_p` - Kubernetes pods.
  - `system_p` - kernel, init, system services (not containerized).
  - `any_p` - any process registered with the SELinux.
  - Service-specific types and typeattributes in service policy files.
- Files:
  - `common_f` - world-rw files, which can be accessed by any process.
  - `protected_f` - mostly files used by specific services, not accessible by other processes (except e.g. machined)
  - `system_f` - files and directories used by the system services, also generally to be specified by precise type and not typeattribute.
  - `system_socket_f` - sockets used for communication between system services, not accessible by workload processes.
  - `device_f`:
    - `common_device_f` - devices not considered protected like GPUs.
    - `protected_device_f` - protected devices like TPM, watchdog timers.
  - `any_f` - any file registered with the SELinux.
  - `filesystem_f` - filesystems, generally used for allowing mount operations.
  - `service_exec_f` - system service executable files.
  - Service-specific types and typeattributes in service policy files.
- General:
  - `any_f_any_p` - any file or any process, the widest typeattribute.
