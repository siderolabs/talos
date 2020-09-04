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

Talos is flexible and can be deployed in a variety of ways, but the easiest way to get started and experiment with the system is to run a local cluster on your laptop or workstation.
There are two options:

- [Run a Docker-based local cluster](/docs/v0.6/en/guides/local/docker) on your Linux or Mac workstation
- [Run a Firecracker micro-VM-based](/docs/v0.6/en/guides/local/firecracker) cluster on your Linux workstation

### System requirements

Talos itself is a low resource OS, but since Talos goal is to run Kubernetes you need to have at least the following hardware requirements:

#### Minimum requirements

<table class="table-auto">
  <thead>
    <tr>
      <th class="px-4 py-2">Role</th>
      <th class="px-4 py-2">Memory</th>
      <th class="px-4 py-2">Cores</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td class="border px-4 py-2">Init/Control Plane</td>
      <td class="border px-4 py-2">2GB</td>
      <td class="border px-4 py-2">2</td>
    </tr>
    <tr class="bg-gray-100">
      <td class="border px-4 py-2">Worker</td>
      <td class="border px-4 py-2">1GB</td>
      <td class="border px-4 py-2">1</td>
    </tr>
  </tbody>
</table>

#### Recommended

<table class="table-auto">
  <thead>
    <tr>
      <th class="px-4 py-2">Role</th>
      <th class="px-4 py-2">Memory</th>
      <th class="px-4 py-2">Cores</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td class="border px-4 py-2">Init/Control Plane</td>
      <td class="border px-4 py-2">4GB</td>
      <td class="border px-4 py-2">4</td>
    </tr>
    <tr class="bg-gray-100">
      <td class="border px-4 py-2">Worker</td>
      <td class="border px-4 py-2">2GB</td>
      <td class="border px-4 py-2">2</td>
    </tr>
  </tbody>
</table>

These requirements are similar to that of kubernetes.
