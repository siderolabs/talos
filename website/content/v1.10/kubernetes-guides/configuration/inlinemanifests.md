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

```yaml
cluster:
  extraManifests:
    - "https://raw.githubusercontent.com/example/repo/main/manifest.yaml"
    - "https://gist.githubusercontent.com/user/gist-id/raw/manifest.yaml"
```

## Resource Ordering Considerations

You must note that although `inlineManifests` and `extraManifests` are defined as arrays, Talos does not guarantee that the items will be applied in the order they appear.

If your resources depend on one another and must be applied in a specific order, define them in a single manifest entry and separate each resource using the `---` YAML document separator.

This ensures they are parsed and applied in the correct order.

Here’s an example:

```yaml
cluster:
  inlineManifests:
    - name: cilium-install
      contents: |
        ---
        apiVersion: v1
        kind: ServiceAccount
        metadata:
          name: cilium-install
          namespace: kube-system
        ---
        apiVersion: rbac.authorization.k8s.io/v1
        kind: ClusterRoleBinding
        metadata:
          name: cilium-install
        roleRef:
          apiGroup: rbac.authorization.k8s.io
          kind: ClusterRole
          name: cluster-admin
        subjects:
        - kind: ServiceAccount
          name: cilium-install
          namespace: kube-system

```

## Example Usecase: Install a GitOps controller with extraManifests

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

## How Talos Handles Manifest Resources During Upgrades

During upgrades, Talos follows a conservative, additive-only approach when processing your `inlineManifests` and `extraManifests`.
Here’s what that means in practice:

* **Creates missing resources**: If a resource defined in your manifests doesn't exist in the cluster, Talos will create it.

* **Preserves existing resources**: Resources that already exist in the cluster are left completely unchanged, regardless of any differences between the current state and the manifest definition.

* **Never deletes resources**: Talos will not remove resources from the cluster, even if they're no longer present in your manifest configuration

This means that any manual changes, updates, or customizations made to these resources after the initial deployment will persist through Talos upgrades.
The upgrade process will not overwrite, downgrade, or interfere with the current state of existing Kubernetes resources.
