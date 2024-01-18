---
title: "How to manage certificate lifetimes with Talos Linux"
aliases:

---

Talos Linux automatically manages and rotates all server side certs for etcd, Kubernetes, and the Talos API.
Note however that the kubelet needs to be restarted at least once a year in order for the certificates to be rotated.
Any upgrade/reboot of the node will suffice for this effect.

Client certs (`talosconfig` and `kubeconfig`) are the user's responsibility.
Each time you download the `kubeconfig` file from a Talos Linux cluster, the client certificate is regenerated giving you a kubeconfig which is valid for a year.

The `talosconfig` file should be renewed at least once a year, using the `talosctl config new` command.
