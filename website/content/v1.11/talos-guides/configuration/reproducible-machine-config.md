---
title: "Reproducible Machine Configuration"
description: "How to reliably and consistently regenerate Talos machine configs from source inputs over time."
---

Talos uses declarative configuration for clusters, but upgrades can cause drift between the declared machine configuration (files you keep in Git) and the deployed configuration (what’s actually running on your nodes).

To prevent this drift, we recommend discarding full declared configuration files and instead using a patch-based workflow to regenerate machine configuration whenever you need them.

To reliably regenerate reproducible machine configurations, that is, configurations that can be recreated consistently from the same source inputs, you’ll need the following inputs:

* **`secrets.yaml`**: The [cluster secrets]({{< relref "../../introduction/prodnotes.md#step-5-generate-secrets-bundle" >}}) generated once at cluster creation.
* **Patch files**: Patches that describe the configuration differences you want from the defaults (e.g. custom networking, node labels, additional arguments). See [Configuration Patches]({{< relref "../configuration/patching.md" >}}) for more information.
* **Cluster name and Kubernetes controlplane endpoint**.
* **Kubernetes version (`--kubernetes-version`)**: The version your cluster runs right now.
* **Talos version (`--talos-version`) contract**: The Talos version you originally used to generate the machine configs. Keep this value fixed to ensure reproducibility.

> **Note**: If you leave the Talos version contract unset, or change it to a newer version, `talosctl gen config` may
> generate a different machine configuration that introduces new fields or defaults that did not exist in your original
> config.
>
> This can silently change cluster behavior and break reproducibility. Only update `--talos-version` when you
> explicitly want to upgrade Talos.

With these inputs ready, follow the workflow below:

## Regenerate Your Machine Configuration

To regenerate your machine configuration:

1. Create new configs using your inputs (`secrets.yaml`, patches, cluster name, and endpoint):

    ```bash
    talosctl gen config <cluster-name> <cluster-endpoint> \
        --with-secrets secrets.yaml \
        --kubernetes-version <kubernetes-version> \
        --talos-version <talos-version-contract> \
        --config-patch @patches/common.yaml \
        --config-patch @patches/controlplane.yaml \
        --config-patch @patches/my-node1.yaml \
        --output-types controlplane \
        --output controlplane-my-node1.yaml
    ```

1. [Apply the generated configs]({{< relref "../../introduction/getting-started.md#step-7-apply-configurations" >}}) to your nodes.

1. Discard the generated configs. Do not commit them, instead, regenerate them when needed.

This workflow prevents drift because version bumps live in regenerated base configs, while your intent remains in small, durable patches.
