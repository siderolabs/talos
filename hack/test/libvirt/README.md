# Integration Testing

## Setup

### Prerequisites

- A linux machine with KVM enabled
- `docker`
- `docker-compose`
- `virt-install`
- `qemu-kvm`
- `yq`

```bash
apt install -y virtinst qemu-kvm
curl -L https://github.com/mikefarah/yq/releases/download/2.4.1/yq_linux_amd64 -o /usr/local/bin/yq
chmod +x /usr/local/bin/yq
```

### Start Matchbox, Dnsmasq, and HAproxy

```bash
docker-compose up
```

> Note: This will run all services in the foreground.

### Create the VMs

```bash
./libvirt.sh up
```

### Getting the Console Logs

```bash
virsh console <VM>
```

### Connecting to the Nodes

#### From the Host

##### Setup DNS

Append the following to `/etc/hosts`:

```text
172.28.1.3 kubernetes.talos.dev
172.28.1.10 control-plane-1.talos.dev
172.28.1.11 control-plane-2.talos.dev
172.28.1.12 control-plane-3.talos.dev
172.28.1.13 worker-1.talos.dev
```

##### Setup `talosctl` and `kubectl`

```bash
export TALOSCONFIG=$PWD/matchbox/assets/talosconfig
export KUBECONFIG=$PWD/matchbox/assets/kubeconfig
```

```bash
talosctl config endpoint 172.28.1.10
talosctl kubeconfig ./matchbox/assets/kubeconfig
```

#### From a Container

```bash
./libvirt.sh workspace
```

```bash
talosctl config endpoint 172.28.1.10
talosctl kubeconfig .
```

#### Verify Connectivity

```bash
talosctl services
kubectl get nodes
```

## Teardown

To teardown the test:

```bash
docker-compose down
./libvirt.sh down
```
