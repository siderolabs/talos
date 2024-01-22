---
title: "FAQs"
weight: 999
description: "Frequently Asked Questions about Talos Linux."
---

<!-- markdownlint-disable MD026 -->

## How is Talos different from other container optimized Linux distros?

Talos integrates tightly with Kubernetes, and is not meant to be a general-purpose operating system.
The most important difference is that Talos is fully controlled by an API via a gRPC interface, instead of an ordinary shell.
We don't ship SSH, and there is no console access.
Removing components such as these has allowed us to dramatically reduce the footprint of Talos, and in turn, improve a number of other areas like security, predictability, reliability, and consistency across platforms.
It's a big change from how operating systems have been managed in the past, but we believe that API-driven OSes are the future.

## Why no shell or SSH?

Since Talos is fully API-driven, all maintenance and debugging operations are possible via the OS API.
We would like for Talos users to start thinking about what a "machine" is in the context of a Kubernetes cluster.
That is, that a Kubernetes _cluster_ can be thought of as one massive machine, and the _nodes_ are merely additional, undifferentiated resources.
We don't want humans to focus on the _nodes_, but rather on the _machine_ that is the Kubernetes cluster.
Should an issue arise at the node level, `talosctl` should provide the necessary tooling to assist in the identification, debugging, and remediation of the issue.
However, the API is based on the Principle of Least Privilege, and exposes only a limited set of methods.
We envision Talos being a great place for the application of [control theory](https://en.wikipedia.org/wiki/Control_theory) in order to provide a self-healing platform.

## Why the name "Talos"?

Talos was an automaton created by the Greek God of the forge to protect the island of Crete.
He would patrol the coast and enforce laws throughout the land.
We felt it was a fitting name for a security focused operating system designed to run Kubernetes.

## Why does Talos rely on a separate configuration from Kubernetes?

The `talosconfig` file contains client credentials to access the Talos Linux API.
Sometimes Kubernetes might be down for a number of reasons (etcd issues, misconfiguration, etc.), while Talos API access will always be available.
The Talos API is a way to access the operating system and fix issues, e.g. fixing access to Kubernetes.
When Talos Linux is running fine, using the Kubernetes APIs (via `kubeconfig`) is all you should need to deploy and manage Kubernetes workloads.

## How does Talos handle certificates?

During the machine config generation process, Talos generates a set of certificate authorities (CAs) that remains valid for 10 years.
Talos is responsible for managing certificates for `etcd`, Talos API (`apid`), node certificates (`kubelet`), and other components.
It also handles the automatic rotation of server-side certificates.

However, client certificates such as `talosconfig` and `kubeconfig` are the user's responsibility, and by default, they have a validity period of 1 year.

To renew the `talosconfig` certificate, the follow [this process]({{< relref "../talos-guides/howto/cert-management" >}}).
To renew `kubeconfig`, use `talosctl kubeconfig` command, and the time-to-live (TTL) is defined in the [configuration]({{< relref "../reference/configuration/#adminkubeconfigconfig" >}}).

## How can I set the timezone of my Talos Linux clusters?

Talos doesn't support timezones, and will always run in UTC.
This ensures consistency of log timestamps for all Talos Linux clusters, simplifying debugging.
Your containers can run with any timezone configuration you desire, but the timezone of Talos Linux is not configurable.

## How do I see Talos kernel configuration?

### Using Talos API

Current kernel config can be read with `talosctl -n <NODE> read /proc/config.gz`.

For example:

```shell
talosctl -n NODE read /proc/config.gz | zgrep E1000
```

### Using GitHub

For `amd64`, see https://github.com/siderolabs/pkgs/blob/main/kernel/build/config-amd64.
Use appropriate branch to see the kernel config matching your Talos release.
