---
title: "Deploying Calico CNI"
description: "In this guide you will learn how to set up Calico CNI on Talos in two mode eBPF and NFtables."
---

This documentation is designed to get you up and running with Talos and Calico CNI. Since both Calico and Talos support multiple networking technologies, you will learn how to run your environment with both the [Calico eBPF dataplane](https://docs.tigera.io/calico/latest/operations/ebpf/enabling-ebpf) and [NFTables](https://docs.tigera.io/calico/latest/getting-started/kubernetes/nftables). Optionally, you can also enable Calico's network [observability stack](https://docs.tigera.io/calico/latest/observability/) to gain insights into your cluster networking and policy behavior.

## Configuring Talos

To install Calico, you first need to disable the default CNI. This can be done by applying a patch file during cluster creation.
The store the following YAML template in a file (`patch.yaml`).

```yaml
cluster:
  network:
    cni:
      name: none
```

After generating the patch file add the `--config-patch` argument to your `talosctl gen config`.

```bash
talosctl gen config \
    my-cluster https://calico-talos.local:6443 \
    --config-patch @patch.yaml
```

## Installing Tigera Operator

Recommended way to install Calico is via `Tigera-operator` manifest. The operator will make sure that all Calico components are always up and running.

> **Note** If you like to install Calico using Helm [checkout this document](https://docs.tigera.io/calico/latest/getting-started/kubernetes/helm).

Use the following command to install the latest Tigera operator.

```bash
kubectl create -f https://docs.tigera.io/calico/latest/manifests/tigera-operator.yaml
```

### Configuring Calico Networking

Calico has a pluggable dataplane architecture that lets you choose the networking technology based on your use case. Networking technology is the backend that allows your nodes to move a packet from a source or destination to your Kubernetes resources.

> **Note** If you like to learn more about the available Calico configurations [checkout this document](https://docs.tigera.io/calico/latest/reference/installation/api).

{{< tabpane text=true >}}
{{% tab header="NFTables" %}}

> **Note**:  Calico also supports iptables backend, if you wish to run Calico in iptables mode change `linuxdataplane` value to `Iptables`.

Use the following command to run Calico with NFTables backend.

```bash
kubectl create -f -<<EOF
# This section includes base Calico installation configuration.
apiVersion: operator.tigera.io/v1
kind: Installation
metadata:
  name: default
spec:
  calicoNetwork:
    bgp: Disabled
    linuxDataplane: Nftables
    ipPools:
    - name: default-ipv4-ippool
      blockSize: 26
      cidr: 10.244.0.0/16
      encapsulation: VXLAN
      natOutgoing: Enabled
      nodeSelector: all()
  kubeletVolumePluginPath: None
---
apiVersion: operator.tigera.io/v1
kind: APIServer
metadata:
  name: default
EOF
```

{{% /tab %}}
{{% tab header="eBPF" %}}

By default, Calico uses the `/var` directory to mount cgroups. However, since this path is not writable in Talos Linux, you need to change it to `/sys/fs/cgroup`.

Use the following command to update the cgroup mount path:

```bash
kubectl create -f -<<EOF
apiVersion: crd.projectcalico.org/v1
kind: FelixConfiguration
metadata:
  name: default
spec:
  cgroupV2Path: "/sys/fs/cgroup"
EOF
```

In eBPF mode, Calico completely replaces the need for kube-proxy by programming all networking logic via eBPF programs. Before disabling kube-proxy, however, you need to ensure that Calico components can reach the API server. This can be done by creating a `kubernetes-services-endpoint` ConfigMap.

> **Note**: In this part we assume you are using [KubePrism]({{< relref "../configuration/kubeprism" >}}) (which is enabled by the default).

```bash
kubectl create -f -<<EOF
kind: ConfigMap
apiVersion: v1
metadata:
  name: kubernetes-services-endpoint
  namespace: tigera-operator
data:
  KUBERNETES_SERVICE_HOST: 'localhost'
  KUBERNETES_SERVICE_PORT: '7445'
EOF
```

You can now safely disable `kube-proxy` by using the following command:

```bash
kubectl patch ds -n kube-system kube-proxy -p '{"spec":{"template":{"spec":{"nodeSelector":{"non-calico": "true"}}}}}'
```

Next, you have to configure Calico:

```bash
kubectl create -f -<<EOF
# This section includes base Calico installation configuration.
apiVersion: operator.tigera.io/v1
kind: Installation
metadata:
  name: default
spec:
  calicoNetwork:
    bgp: Disabled
    linuxDataplane: BPF
    ipPools:
    - name: default-ipv4-ippool
      blockSize: 26
      cidr: 10.244.0.0/16
      encapsulation: VXLAN
      natOutgoing: Enabled
      nodeSelector: all()
  kubeletVolumePluginPath: None
EOF
```

{{% /tab %}}
{{< /tabpane >}}

## Deploy Calico Whisker Network Observability Stack

Use the following command to enable Calico observability stack:

```bash
kubectl create -f -<<EOF
# Configures the Calico Goldmane flow aggregator.
apiVersion: operator.tigera.io/v1
kind: Goldmane
metadata:
  name: default
---
# Configures the Calico Whisker observability UI.
apiVersion: operator.tigera.io/v1
kind: Whisker
metadata:
  name: default
EOF
```

Use the following command to access Calico Whisker:

```bash
kubectl port-forward -n calico-system service/whisker 8081:8081
```

Fire up a browser and point it to `localhost:8081` to observe your policies and network flows.

## Next steps

- Enable Calico Prometheus and Grafana integrations, click here to [learn more](https://docs.tigera.io/calico/latest/operations/monitor/).

## Considerations

**In eBPF mode**, if you cannot disable kube-proxy for any reason please make sure to adjust `BPFKubeProxyIptablesCleanupEnabled` to `false`.
This can be done with kubectl as follows:

```bash
kubectl patch felixconfiguration default --patch='{"spec": {"bpfKubeProxyIptablesCleanupEnabled": false}}'
```
