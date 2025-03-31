---
title: "Developing Talos"
description: "Learn how to set up a development environment for local testing and hacking on Talos itself!"
aliases:
  - ../learn-more/developing-talos
---

This guide outlines steps and tricks to develop Talos operating systems and related components.
The guide assumes Linux operating system on the development host.
Some steps might work under Mac OS X, but using Linux is highly advised.

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
make imager IMAGE_REGISTRY=127.0.0.1:5005 PUSH=true
make installer IMAGE_REGISTRY=127.0.0.1:5005
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
sudo --preserve-env=HOME _out/talosctl-linux-amd64 cluster create \
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
- `--with-bootloader=false` disables boot from disk (Talos will always boot from `_out/vmlinuz-amd64` and `_out/initramfs-amd64.xz`).
  This speeds up development cycle a lot - no need to rebuild installer and perform install, rebooting is enough to get new code.

> Note: as boot loader is not used, it's not necessary to  rebuild `installer` each time (old image is fine), but sometimes it's needed (when configuration changes are done and old installer doesn't validate the config).
>
> `talosctl cluster create` derives Talos machine configuration version from the install image tag, so sometimes early in the development cycle (when new minor tag is not released yet), machine config version can be overridden with `--talos-version={{< version >}}`.

If the `--with-bootloader=false` flag is not enabled, for Talos cluster to pick up new changes to the code (in `initramfs`), it will require a Talos upgrade (so new `installer` should be built).
With `--with-bootloader=false` flag, Talos always boots from `initramfs` in `_out/` directory, so simple reboot is enough to pick up new code changes.

If the installation flow needs to be tested, `--with-bootloader=false` shouldn't be used.

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
