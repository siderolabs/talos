---
title: "inlineManifests and extraManifests"
description: "Learn what inlineManifests and extraManifests are, how they differ, and why they matter."
---

`inlineManifests` and `extraManifests` allow you to automatically apply Kubernetes resources to your cluster during installation and upgrades.

Both are designed to automate the provisioning of components like CNIs and other static infrastructure, but they differ in how the manifest content is sourced and applied.
They are not meant for deploying applications or frequently changing services.
For those, it's better to use a GitOps or CI/CD tool.

## inlineManifests

`inlineManifests` are defined directly within the machine configuration file.
The YAML content is embedded inside the `inlineManifests` section, making it ideal for tightly coupled resources that need to be provisioned as soon as the node boots up.

Here’s an example of how to configure a cluster using an `inlineManifest`:

```yaml
cluster:
  inlineManifests:
    - name: my-app
      contents: |
        apiVersion: apps/v1
        kind: Deployment
        metadata:
          name: my-application
        spec:
          # ... deployment specification
```

## extraManifests

`extraManifests` are Kubernetes manifests fetched from external, unauthenticated HTTP sources such as GitHub, raw file servers, or gists.
You define them in the `extraManifests` section of the machine configuration.
They’re best suited for shared, versioned, or centrally managed resources.
These manifests are pulled directly by the node during configuration.
If the node doesn’t have network access to the HTTP endpoint hosting the manifest the installation will fail.

Similarly, if the endpoint is down or returns an error, the manifest will not be applied, and the machine configuration will fail as a result.

Here’s how to reference `extraManifests`:

```bash
cluster:
  extraManifests:
    - "https://raw.githubusercontent.com/example/repo/main/manifest.yaml"
    - "https://gist.githubusercontent.com/user/gist-id/raw/manifest.yaml"
```

## Usage Example

A common use case for `inlineManifests` or `extraManifests` is to install a GitOps controller like Flux or ArgoCD.
Once the controller is running, it connects to your Git repository and automatically applies the rest of your Kubernetes configuration.

Here's how to install the Flux GitOps controller using an `extraManifest`:

1. Create a patch file named `flux-extra-manifest.yaml` that automatically downloads and applies the Flux installation manifest from GitHub:

    ```shell
    cat << EOF > flux-extra-manifest.yaml
    cluster:
    extraManifests:
        - "https://github.com/fluxcd/flux2/releases/latest/download/install.yaml"
    EOF
    ```

1. Create a `CP_IPS` variable that contains the IP addresses of your control plane nodes:

    ```bash
    CP_IPS="<control-plane-ip-1>,<control-plane-ip-2>,<control-plane-ip-3>"
    ```

1. Run this command to export your `TALOSCONFIG` variable.
You can skip this step if you've already done it:

    ```bash
    mkdir -p ~/.talos
    cp ./talosconfig ~/.talos/config
    ```

1. Apply the `flux-extra-manifest.yaml` patch to your control plane nodes:

    ```bash
    talosctl patch machineconfig \
    --patch @flux-extra-manifest.yaml \
    --endpoints $CP_IPS \
    --nodes $CP_IPS
    ```

1. Reboot the nodes.
Note that if you have only one control plane node, rebooting it will cause cluster downtime.

    ```bash
    for NODE in $(echo "${CP_IPS}" | tr ',' ' '); do
        echo "Rebooting control plane node: $NODE"
        talosctl reboot --endpoints "$NODE" --nodes "$NODE" --wait
    done
    ```

1. Wait a few seconds and check for the Flux pods:

    ```bash
    kubectl get pods -n flux-system -w
    ```

## Omni Patches

You can also apply `inlineManifests` or `extraManifests` patches to Talos clusters managed by Omni.

Refer to [Create a Patch For Cluster Machines](https://omni.siderolabs.com/how-to-guides/create-a-patch-for-cluster-machines?q=inline+manifest&ask=true) to learn how to create and apply the patches.

## Summary: inlineManifests vs extraManifests

Here’s a quick overview of the key differences between `inlineManifests` and `extraManifests`:

|                        | `inlineManifests`                              | `extraManifests`                                             |
| ---------------------- | ---------------------------------------------- | ------------------------------------------------------------ |
| Source                 | Defined directly in the machine configuration  | Pulled from external URLs (GitHub gists, web servers, gists) |
| Configuration Location | Under the `inlineManifests` section.           | Under the `extraManifests` section                           |
| Usecase                | Early bootstrapping of critical resources      | For reusable, version-controlled, or shared manifests        |
| Benefits               | No external dependencies                       | Centrally managed                                            |
| Disadvantages          | Difficult to maintain and format embedded YAML | Requires external HTTP server                                |

## Key Considerations

One important thing to note is that Talos treats upgrades the same way it handles fresh installs.
During an upgrade, Talos installs the new version onto a separate partition and then switches to it.
As a result, any `inlineManifests` or `extraManifests` defined in the machine configuration will be re-applied.
This can unintentionally overwrite or downgrade components that were manually updated during the cluster's lifetime.

### Recommendations

To manage these current limitations, we recommend that you:

- Avoid placing version-sensitive applications in `inlineManifests` or `extraManifests`, unless absolutely necessary.
- Only include manifests for components that are unlikely to change throughout the life of your cluster.
- Apply the manifests once, then remove them from the machine configuration and manage them as standard Kubernetes resources going forward.
