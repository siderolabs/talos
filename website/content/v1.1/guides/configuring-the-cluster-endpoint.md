---
title: "Configuring the Cluster Endpoint"
description: ""
---

In this section, we will step through the configuration of a Talos based Kubernetes cluster.
There are three major components we will configure:

- `apid` and `talosctl`
- the master nodes
- the worker nodes

Talos enforces a high level of security by using mutual TLS for authentication and authorization.

We recommend that the configuration of Talos be performed by a cluster owner.
A cluster owner should be a person of authority within an organization, perhaps a director, manager, or senior member of a team.
They are responsible for storing the root CA, and distributing the PKI for authorized cluster administrators.

### Recommended settings

Talos runs great out of the box, but if you tweak some minor settings it will make your life
a lot easier in the future.
This is not a requirement, but rather a document to explain some key settings.

#### Endpoint

To configure the `talosctl` endpoint, it is recommended you use a resolvable DNS name.
This way, if you decide to upgrade to a multi-controlplane cluster you only have to add the ip address to the hostname configuration.
The configuration can either be done on a Loadbalancer, or simply trough DNS.

For example:

> This is in the config file for the cluster e.g. controlplane.yaml and worker.yaml.
> for more details, please see: [v1alpha1 endpoint configuration](../../reference/configuration/#controlplaneconfig)

```yaml
.....
cluster:
  controlPlane:
    endpoint: https://endpoint.example.local:6443
.....
```

If you have a DNS name as the endpoint, you can upgrade your talos cluster with multiple controlplanes in the future (if you don't have a multi-controlplane setup from the start)
Using a DNS name generates the corresponding Certificates (Kubernetes and Talos) for the correct hostname.
