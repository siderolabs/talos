---
title: Introduction to Talos
---

Welcome to the Talos documentation!
Talos is an open source platform to host and maintain Kubernetes clusters.
It includes a purpose-built operating system and associated management tools.
It can run on all major cloud providers, virtualization platforms, and bare metal hardware.

All system management is done via an API, and there is no shell or interactive console.
Some of the capabilities and benefits provided by Talos include:

- **Security**: Talos reduces your attack surface by practicing the Principle of Least Privilege (PoLP) and by securing the API with mutual TLS (mTLS) authentication.
- **Predictability**: Talos eliminates unneeded variables and reduces unknown factors in your environment by employing immutable infrastructure ideology.
- **Evolvability**: Talos simplifies your architecture and increases your ability to easily accommodate future changes.

Talos is flexible and can be deployed in a variety of ways, but the easiest way to get started and experiment with the system is to run a local cluster on your laptop or workstation using Docker.

- [Run a Docker-based local cluster](/docs/v0.3/en/guides/getting-started)
