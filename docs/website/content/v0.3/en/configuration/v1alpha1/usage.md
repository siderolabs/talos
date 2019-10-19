---
title: 'v1alpha1 Usage'
---

Talos enforces a high level of security by using mutual TLS for authentication and authorization.

We recommend that the configuration of Talos be performed by a cluster owner.
A cluster owner should be a person of authority within an organization, perhaps a director, manager, or senior member of a team.
They are responsible for storing the root CA, and distributing the PKI for authorized cluster administrators.

## Generate base configuration

We can generate a basic configuration using `osctl`.
This configuration is enough to get started with, however it can be customized as needed.

```bash
osctl config generate --version v1alpha1 <cluster name> <master ip>[,<master ip>...]
```

This command will generate a yaml config per master node, a worker config, and a talosconfig.

## Example of generated master-1.yaml

```bash
osctl config generate --version v1alpha1 cluster.local 1.2.3.4,2.3.4.5,3.4.5.6
```

```yaml
version: v1alpha1
machine:
  type: init
  token: hmh6z7.nzk7is2wobd9zlgh
  ca:
    crt: LS0tLS1CRUd...
    key: LS0tLS1CRUd...
  kubelet: {}
  network: {}
cluster:
  controlPlane:
    ips:
      - 1.2.3.4
      - 2.3.4.5
      - 3.4.5.6
  clusterName: cluster.local
  network:
    dnsDomain: cluster.local
    podSubnets:
      - 10.244.0.0/16
    serviceSubnets:
      - 10.96.0.0/12
  token: ndg6bi.cfj4sk82nddtr2hv
  ca:
    crt: LS0tLS1CR...
    key: LS0tLS1CR...
  apiServer:
    certSANs:
      - 127.0.0.1
      - ::1
      - 1.2.3.4
      - 2.3.4.5
      - 3.4.5.6
  controllerManager: {}
  scheduler: {}
  etcd: {}
```

The above configuration can be customized as needed by using the following [reference guide](/docs/configuration/v1alpha1-reference/).
