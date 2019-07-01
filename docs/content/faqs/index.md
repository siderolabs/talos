---
title: FAQs
---

**Why "Talos"?**

> Talos was an automaton created by the Greek God of the forge to protect the island of Crete.
> He would patrol the coast and enforce laws throughout the land.
> We felt it was a fitting name for a security focused operating system designed to run Kubernetes.

**Why no shell or SSH?**

> We would like for Talos users to start thinking about what a "machine" is in the context of a Kubernetes cluster.
> That is that a Kubernetes _cluster_ can be thought of as one massive machine and the _nodes_ merely as additional resources.
> We don't want humans to focus on the _nodes_, but rather the _machine_ that is the Kubernetes cluster.
> Should an issue arise at the node level, osctl should provide the necessary tooling to assist in the identification, debugging, and remediation of the issue.
> However, the API is based on the Principle of Least Privilege, and exposes only a limited set of methods.
> We aren't quite there yet, but we envision Talos being a great place for the application of [control theory](https://en.wikipedia.org/wiki/Control_theory) in order to provide a self-healing platform.

**How is Talos different than CoreOS/RancherOS/Linuxkit?**

> Talos is similar in many ways, but there are some differences that make it unique.
> You can imagine Talos as a container image, in that it is immutable and built with a single purpose in mind.
> In this case, that purpose is Kubernetes.
> Talos tightly integrates with Kubernetes, and is not meant to be a general use operating system.
> This allows us to dramatically decrease the footprint of Talos, and in turn improve a number of other areas like security, predictability, and reliability.
> In addition to this, interaction with the host is done through a secure gRPC API.
> If you want to run Kubernetes with zero cruft, Talos is the perfect fit.
