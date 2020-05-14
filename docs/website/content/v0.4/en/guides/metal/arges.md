---
title: "Arges: Flexible Provisioning for Kubernetes"
---

> Arges is an alpha-level project and is in active development.
> If you need help or have questions, please get in touch with us by Slack or GitHub!
> If you want to skip right to the code, check it out on GitHub: [talos-systems/arges](https://github.com/talos-systems/arges)

The goal of the Arges project is to provide Talos users with a robust and reliable way to build and manage bare metal Talos-based Kubernetes clusters, as well as manage cloud-based clusters.
We've tried to achieve this by building out a set of tools to help solve the traditional datacenter bootstrapping problems.
These tools include an asset management server, a metadata server, and a pair of Cluster API-aware providers for infrastructure provisioning and config generation.

<img src="/images/arges-arch.png" width="700">

Since Arges is currently in active development, the best place to start will be the [project README](https://github.com/talos-systems/arges/blob/master/README.md).
In the GitHub repository, you can find an [example project](https://github.com/talos-systems/arges/blob/master/examples/README.md) showing one method of deployment.
