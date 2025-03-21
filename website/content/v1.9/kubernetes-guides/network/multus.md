---
title: "Multus CNI"
description: "A brief instruction on howto use Multus on Talos Linux"
---

[Multus CNI](https://github.com/k8snetworkplumbingwg/multus-cni) is a container network interface (CNI) plugin for Kubernetes that enables attaching multiple network interfaces to pods.
Typically, in Kubernetes each pod only has one network interface (apart from a loopback) -- with Multus you can create a multi-homed pod that has multiple interfaces.
This is accomplished by Multus acting as a "meta-plugin", a CNI plugin that can call multiple other CNI plugins.

## Installation

Multus can be deployed by simply applying the `thick` `DaemonSet` with `kubectl`.

```bash
kubectl apply -f https://raw.githubusercontent.com/k8snetworkplumbingwg/multus-cni/master/deployments/multus-daemonset-thick.yml
```

This will create a `DaemonSet` and a CRD: `NetworkAttachmentDefinition`.
This can be used to specify your network configuration.

## Configuration

### Patching the `DaemonSet`

For Multus to properly work with Talos a change need to be made to the `DaemonSet`.
Instead of of mounting the volume called `host-run-netns` on `/run/netns` it has to be mounted on `/var/run/netns`.

Edit the `DaemonSet` and change the volume `host-run-netns` from `/run/netns` to `/var/run/netns`.

```yaml
...
        - name: host-run-netns
          hostPath:
            path: /var/run/netns/
```

Failing to do so will leave your cluster crippled.
Running pods will remain running but new pods and deployments will give you the following error in the events:

```text
  Normal   Scheduled               3s    default-scheduler  Successfully assigned virtualmachines/samplepod to virt2
  Warning  FailedCreatePodSandBox  3s    kubelet            Failed to create pod sandbox: rpc error: code = Unknown desc = failed to setup network for sandbox "3a6a58386dfbf2471a6f86bd41e4e9a32aac54ccccd1943742cb67d1e9c58b5b": plugin type="multus-shim" name="multus-cni-network" failed (add): CmdAdd (shim): CNI request failed with status 400: 'ContainerID:"3a6a58386dfbf2471a6f86bd41e4e9a32aac54ccccd1943742cb67d1e9c58b5b" Netns:"/var/run/netns/cni-1d80f6e3-fdab-4505-eb83-7deb17431293" IfName:"eth0" Args:"IgnoreUnknown=1;K8S_POD_NAMESPACE=virtualmachines;K8S_POD_NAME=samplepod;K8S_POD_INFRA_CONTAINER_ID=3a6a58386dfbf2471a6f86bd41e4e9a32aac54ccccd1943742cb67d1e9c58b5b;K8S_POD_UID=8304765e-fd7e-4968-9144-c42c53be04f4" Path:"" ERRORED: error configuring pod [virtualmachines/samplepod] networking: [virtualmachines/samplepod/8304765e-fd7e-4968-9144-c42c53be04f4:cbr0]: error adding container to network "cbr0": DelegateAdd: cannot set "" interface name to "eth0": validateIfName: no net namespace /var/run/netns/cni-1d80f6e3-fdab-4505-eb83-7deb17431293 found: failed to Statfs "/var/run/netns/cni-1d80f6e3-fdab-4505-eb83-7deb17431293": no such file or directory
': StdinData: {"capabilities":{"portMappings":true},"clusterNetwork":"/host/etc/cni/net.d/10-flannel.conflist","cniVersion":"0.3.1","logLevel":"verbose","logToStderr":true,"name":"multus-cni-network","type":"multus-shim"}
```

As of March 21, 2025, Multus has a [bug](https://github.com/k8snetworkplumbingwg/multus-cni/issues/1221) in the `install-multus-binary` container that can be lead to race problems after a node reboot.
To prevent this issue, it is necessary to patch this container.
Set the following command to the `install-multus-binary` container.

```yaml
      initContainers:
        - name: install-multus-binary
          command:
            - "/usr/src/multus-cni/bin/install_multus"
            - "-d"
            - "/host/opt/cni/bin"
            - "-t"
            - "thick"
```

### Creating your `NetworkAttachmentDefinition`

The `NetworkAttachmentDefinition` configuration is used to define your bridge where your second pod interface needs to be attached to.

```yaml
apiVersion: "k8s.cni.cncf.io/v1"
kind: NetworkAttachmentDefinition
metadata:
  name: macvlan-conf
spec:
  config: '{
      "cniVersion": "0.3.0",
      "type": "macvlan",
      "master": "eth0",
      "mode": "bridge",
      "ipam": {
        "type": "host-local",
        "subnet": "192.168.1.0/24",
        "rangeStart": "192.168.1.200",
        "rangeEnd": "192.168.1.216",
        "routes": [
          { "dst": "0.0.0.0/0" }
        ],
        "gateway": "192.168.1.1"
      }
    }'
```

In this example `macvlan` is used as a bridge type.
There are 3 types of bridges: `bridge`, `macvlan` and `ipvlan`:

1. `bridge` is a way to connect two Ethernet segments together in a protocol-independent way.
   Packets are forwarded based on Ethernet address, rather than IP address (like a router).
   Since forwarding is done at Layer 2, all protocols can go transparently through a bridge.
   In terms of containers or virtual machines, a bridge can also be used to connect the virtual interfaces of each container/VM to the host network, allowing them to communicate.

2. `macvlan` is a driver that makes it possible to create virtual network interfaces that appear as distinct physical devices each with unique MAC addresses.
  The underlying interface can route traffic to each of these virtual interfaces separately, as if they were separate physical devices.
  This means that each macvlan interface can have its own IP subnet and routing.
  Macvlan interfaces are ideal for situations where containers or virtual machines require the same network access as the host system.

3. `ipvlan` is similar to `macvlan`, with the key difference being that ipvlan shares the parent's MAC address, which requires less configuration from the networking equipment.
   This makes deployments simpler in certain situations where MAC address control or limits are in place.
   It offers two operational modes: L2 mode (the default) where it behaves similarly to a MACVLAN, and L3 mode for routing based traffic isolation (rather than bridged).

When using the `bridge` interface you must also configure a bridge on your Talos nodes.
That can be done by updating Talos Linux machine configuration:

```yaml
machine:
    network:
      interfaces:
      - interface: br0
        addresses:
          - 172.16.1.60/24
        bridge:
          stp:
            enabled: true
          interfaces:
              - eno1 # This must be changed to your matching interface name
        routes:
            - network: 0.0.0.0/0 # The route's network (destination).
              gateway: 172.16.1.254 # The route's gateway (if empty, creates link scope route).
              metric: 1024 # The optional metric for the route.
```

More information about the configuration of bridges can be found [here](https://github.com/k8snetworkplumbingwg/multus-cni/tree/master/docs)

## Attaching the `NetworkAttachmentDefinition` to your `Pod` or `Deployment`

After the `NetworkAttachmentDefinition` is configured, you can attach that interface to your your `Deployment` or `Pod`.
In this example we use a pod:

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: samplepod
  annotations:
    k8s.v1.cni.cncf.io/networks: macvlan-conf
spec:
  containers:
  - name: samplepod
    command: ["/bin/ash", "-c", "trap : TERM INT; sleep infinity & wait"]
    image: alpine
```

## Notes on using KubeVirt in combination with Multus

If you would like to use KubeVirt and expose your virtual machine to the outside world with Multus, make sure to configure a `bridge` instead of `macvlan` or `ipvlan`, because that doesn't work, according to the KubeVirt [Documentation](https://kubevirt.io/user-guide/virtual_machines/interfaces_and_networks/#invalid-cnis-for-secondary-networks).

> Invalid CNIs for secondary networks
> The following list of CNIs is known not to work for bridge interfaces - which are most common for secondary interfaces.
>
> * macvlan
> * ipvlan
>
> The reason is similar: the bridge interface type moves the pod interface MAC address to the VM, leaving the pod interface with a different address.
> The aforementioned CNIs require the pod interface to have the original MAC address.

## Notes on using Cilium in combination with Multus

### CNI reference plugins

Cilium does not ship the CNI reference plugins, which most multus setups are expecting (e.g. macvlan).
This can be addressed by extending the daemonset with an additional init-container, setting them up, e.g. using the following kustomize strategic-merge patch:

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: kube-multus-ds
  namespace: kube-system
spec:
  template:
    spec:
      initContainers:
      - command:
        - /install-cni.sh
        image: ghcr.io/siderolabs/install-cni:v1.7.0  # adapt to your talos version
        name: install-cni
        securityContext:
          privileged: true
        volumeMounts:
        - mountPath: /host/opt/cni/bin
          mountPropagation: Bidirectional
          name: cnibin
```

### Exclusive CNI

By default, Cilium is an exclusive CNI, meaning it removes other CNI configuration files.
However, when using Multus, this behavior needs to be disabled.
To do so, set the Helm variable `cni.exclusive=false`.
For more information, refer to the [Cilium documentation](https://docs.cilium.io/en/stable/network/kubernetes/configuration/#adjusting-cni-configuration).

## Notes on ARM64 nodes

The official images (as of 29.07.24) are built incorrectly for ARM64 ([ref](https://github.com/k8snetworkplumbingwg/multus-cni/issues/1251)).
Self-building them is an adequate workaround for now.
