---
title: 'Configuration'
---

In this section, we will step through the configuration of a Talos based Kubernetes cluster.
There are three major components we will configure:

- `osd` and `osctl`
- the master nodes
- the worker nodes

Talos enforces a high level of security by using mutual TLS for authentication and authorization.

We recommend that the configuration of Talos be performed by a cluster owner.
A cluster owner should be a person of authority within an organization, perhaps a director, manager, or senior member of a team.
They are responsible for storing the root CA, and distributing the PKI for authorized cluster administrators.
