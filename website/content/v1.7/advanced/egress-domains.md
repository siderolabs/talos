---
title: "Egress Domains"
description: "Allowing outbound access for installing Talos"
aliases:
  - ../guides/egress-domains
---

For some more constrained environments, it is important to whitelist only specific domains for outbound internet access.
These rules will need to be updated to allow for certain domains if the user wishes to still install and bootstrap Talos from public sources.
That said, users should also note that all of the following components can be mirrored locally with an internal registry, as well as a self-hosted [discovery service](https://github.com/siderolabs/discovery-service) and [image factory](https://github.com/siderolabs/image-factory).

The following list of egress domains was tested using a Fortinet FortiGate Next-Generation Firewall to confirm that Talos was installed, bootstrapped, and Kubernetes was fully up and running.
The FortiGate allows for passing in wildcard domains and will handle resolution of those domains to defined IPs automatically.
All traffic is HTTPS over port 443.

Discovery Service:

- discovery.talos.dev

Image Factory:

- factory.talos.dev
- *.azurefd.net (Azure Front Door for serving cached assets)

Google Container Registry / Google Artifact Registry (GCR/GAR):

- gcr.io
- storage.googleapis.com (backing blob storage for images)
- *.pkg.dev (backing blob storage for images)

Github Container Registry (GHCR)

- ghcr.io
- *.githubusercontent.com (backing blob storage for images)

Kubernetes Registry (k8s.io)

- registry.k8s.io
- *.s3.dualstack.us-east-1.amazonaws.com (backing blob storage for images)

> Note: In this testing, DNS and NTP servers were updated to use those services that are built-in to the FortiGate.
        These may also need to be allowed if the user cannot make use of internal services.
        Additionally,these rules only cover that which is required for Talos to be fully installed and running.
        There may be other domains like docker.io that must be allowed for non-default CNIs or workload container images.
