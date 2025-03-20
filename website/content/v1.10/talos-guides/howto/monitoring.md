---
title: "How to configure Talos for metric scraping"
description: "This how-to explains how to configure Talos to allow scraping with Prometheus."
aliases:

---

This was originally posted on Github in reply to an ongoing discussion. You can find it here:
[Link to Github Discussion](https://github.com/siderolabs/talos/discussions/7214#discussioncomment-11709688 "How to get etcd metrics")

In order for Prometheus to succesfully scrape metrics from etcd, Controller Manager and the Scheduler, we need to make a few changes in the machine config and adjust Helm values for `kube-prometheus-stack`.

This how-to is written under the assumption that you have a working deployment of the `kube-prometheus-stack` community project. If you have another Prometheus deployment you may need to make adjustments to suit your particular setup.

Create a patch for the control planes and save it as `etcd_metrics_patch.yaml`:

```yaml
- op: add
  path: /cluster/etcd/extraArgs
  value:
    listen-metrics-urls: https://0.0.0.0:2379
- op: add
  path: /cluster/controllerManager/extraArgs
  value:
    bind-address: 0.0.0.0
- op: add
  path: /cluster/scheduler/extraArgs
  value:
    bind-address: 0.0.0.0
```

Patch the control plane nodes with:
`talosctl patch mc -n 10.0.0.1 --patch @etcd_metrics_patch.yaml`

And repeat with the IP address for each control plane node.

For Prometheus scrape jobs to succesfully read from `etcd`, it requires certificates to authenticate. We can get those by running the following commands:

```sh
talosctl get etcdrootsecret -o yaml
```
Expected output:
```yaml
spec:
    etcdCA:
        LS0t....LS0K
```

```sh
talosctl get etcdsecret  -o yaml
```
Expected output:
```yaml
spec:
    etcd:
        crt: LS0t....LS0K
        key: LS0t....=
```

The strings we need are the values from `etcdCA`, `etcd.crt` and `etc.key`. They are base64 encoded and can be used without the need to decode as the secret we will create also takes base64 encoded string values. In other words, just copy/paste into the new secret which we will apply in the namespace where Prometheus is running. By default, this will be the namespace `monitoring` and the template below assumes that. If yours is different make sure to edit the `metadata`.

Create a new secret and save it as `etcd-secret.yaml` and edit it with your etc CA and cert values:

```yaml
---
apiVersion: v1
kind: Secret
metadata:
  name: etcd-client-cert
  namespace: monitoring
type: Opaque
data:
  etcd-ca.crt:
    LS0t....LS0K
  etcd-client.crt:
    LS0t....LS0K
  etcd-client-key.key:
    LS0t....=
```
`kubectl apply -f etcd-secret.yaml`.

In your `talos-custom-values.yaml` (example below) for the helm deployment of `kube-prometheus-stack`, add or change the following parts. Make sure to replace the IP's with your control planes. Also read the comment for kube-proxy to decide whether to enable that or not:

```yaml
kubeControllerManager:
  endpoints:
    - 10.0.0.1
    - 10.0.0.2
    - 10.0.0.3

kubeEtcd:
  endpoints:
    - 10.0.0.1
    - 10.0.0.2
    - 10.0.0.3
  service:
    selector:
      component: etcd
  serviceMonitor:
    scheme: https
    insecureSkipVerify: false
    serverName: "localhost"
    caFile: "/etc/prometheus/secrets/etcd-client-cert/etcd-ca.crt"
    certFile: "/etc/prometheus/secrets/etcd-client-cert/etcd-client.crt"
    keyFile: "/etc/prometheus/secrets/etcd-client-cert/etcd-client-key.key"

## In case you run a kube-proxy replacement (like Cilium kube-proxy replacement) you need to set enabled: false or comment this out. This is for Kubernetes kube-proxy scraping only and will not work on proxy replacements.
kubeProxy:
	  enabled: true
	  endpoints:
    - 10.0.0.1
    - 10.0.0.2
    - 10.0.0.3
      
kubeScheduler:
  endpoints:
    - 10.0.0.1
    - 10.0.0.2
    - 10.0.0.3

prometheus:
  prometheusSpec:
    secrets:
      - etcd-client-cert
```

Note stating the obvious: the values above are probably not enough for a complete and succesful deployment of `kube-prometheus-stack`. These are only the additional changes that you need to make this particular scraping work. Make sure you have a working setup before applying these, or integrate them into your values for a new setup.

To apply the above changes to an already running `kube-prometheus-stack`, you can use a command similar to this:
```sh
helm upgrade kube-prometheus-stack prometheus-community/kube-prometheus-stack --namespace monitoring --reuse-values --values talos-custom-values.yaml
```