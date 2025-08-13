---
title: "Configuration Patches"
description: "In this guide, we'll patch the generated machine configuration."
---

Talos generates machine configuration for two types of machines: controlplane and worker machines.
Many configuration options can be adjusted using `talosctl gen config` but not all of them.
Configuration patching allows modifying machine configuration to fit it for the cluster or a specific machine.

## Configuration Patch Formats

Talos supports two configuration patch formats:

- strategic merge patches
- RFC6902 (JSON patches)

Strategic merge patches are the easiest to use, but JSON patches allow more precise configuration adjustments.

> Note: Talos 1.5+ supports [multi-document machine configuration]({{< relref "../../reference/configuration" >}}).
> JSON patches don't support multi-document machine configuration, while strategic merge patches do.

### Strategic Merge patches

Strategic merge patches look like incomplete machine configuration files:

```yaml
machine:
  network:
    hostname: worker1
```

When applied to the machine configuration, the patch gets merged with the respective section of the machine configuration:

```yaml
machine:
  network:
    interfaces:
      - interface: eth0
        addresses:
          - 10.0.0.2/24
    hostname: worker1
```

In general, machine configuration contents are merged with the contents of the strategic merge patch, with strategic merge patch
values overriding machine configuration values.
There are some special rules:

- If the field value is a list, the patch value is appended to the list, with the following exceptions:
  - values of the fields `cluster.network.podSubnets` and `cluster.network.serviceSubnets` are overwritten on merge
  - `network.interfaces` section is merged with the value in the machine config if there is a match on `interface:` or `deviceSelector:` keys
  - `network.interfaces.vlans` section is merged with the value in the machine config if there is a match on the `vlanId:` key
  - `cluster.apiServer.auditPolicy` value is replaced on merge
  - `ExtensionServiceConfig.configFiles` section is merged matching on `mountPath` (replacing `content` if matches)

When patching a [multi-document machine configuration]({{< relref "../../reference/configuration" >}}), following rules apply:

- for each document in the patch, the document is merged with the respective document in the machine configuration (matching by `kind`, `apiVersion` and `name` for named documents)
- if the patch document doesn't exist in the machine configuration, it is appended to the machine configuration

The strategic merge patch itself might be a multi-document YAML, and each document will be applied as a patch to the base machine configuration.
Keep in mind that you can't patch the same document multiple times with the same patch.

You can also delete parts from the configuration using `$patch: delete` syntax similar to the
[Kubernetes](https://github.com/kubernetes/community/blob/master/contributors/devel/sig-api-machinery/strategic-merge-patch.md#delete-directive)
strategic merge patch.

For example, with configuration:

```yaml
machine:
  network:
    interfaces:
      - interface: eth0
        addresses:
          - 10.0.0.2/24
    hostname: worker1
```

and patch document:

```yaml
machine:
  network:
    interfaces:
    - interface: eth0
      $patch: delete
    hostname: worker1
```

The resulting configuration will be:

```yaml
machine:
  network:
    hostname: worker1
```

You can also delete entire docs (but not the main `v1alpha1` configuration!) using this syntax:

```yaml
apiVersion: v1alpha1
kind: SideroLinkConfig
$patch: delete
---
apiVersion: v1alpha1
kind: ExtensionServiceConfig
name: foo
$patch: delete
```

This will remove the documents `SideroLinkConfig` and `ExtensionServiceConfig` with name `foo` from the configuration.

### RFC6902 (JSON Patches)

[JSON patches](https://jsonpatch.com/) can be written either in JSON or YAML format.
A proper JSON patch requires an `op` field that depends on the machine configuration contents: whether the path already exists or not.

For example, the strategic merge patch from the previous section can be written either as:

```yaml
- op: replace
  path: /machine/network/hostname
  value: worker1
```

or:

```yaml
- op: add
  path: /machine/network/hostname
  value: worker1
```

The correct `op` depends on whether the `/machine/network/hostname` section exists already in the machine config or not.

## Examples

### Machine Network

Base machine configuration:

```yaml
# ...
machine:
  network:
    interfaces:
      - interface: eth0
        dhcp: false
        addresses:
          - 192.168.10.3/24
```

The goal is to add a virtual IP `192.168.10.50` to the `eth0` interface and add another interface `eth1` with DHCP enabled.

<!-- markdownlint-disable MD007 -->
<!-- markdownlint-disable MD032 -->
<!-- markdownlint-disable MD025 -->

{{< tabpane lang="yaml" right=true >}}
{{< tab header="Strategic merge patch" >}}
machine:
  network:
    interfaces:
      - interface: eth0
        vip:
          ip: 192.168.10.50
      - interface: eth1
        dhcp: true
{{< /tab >}}
{{< tab header="JSON patch" >}}
- op: add
  path: /machine/network/interfaces/0/vip
  value:
    ip: 192.168.10.50
- op: add
  path: /machine/network/interfaces/-
  value:
    interface: eth1
    dhcp: true
{{< /tab >}}
{{< /tabpane >}}

Patched machine configuration:

```yaml
machine:
  network:
    interfaces:
      - interface: eth0
        dhcp: false
        addresses:
          - 192.168.10.3/24
        vip:
          ip: 192.168.10.50
      - interface: eth1
        dhcp: true
```

### Cluster Network

Base machine configuration:

```yaml
cluster:
  network:
    dnsDomain: cluster.local
    podSubnets:
      - 10.244.0.0/16
    serviceSubnets:
      - 10.96.0.0/12
```

The goal is to update pod and service subnets and disable default CNI (Flannel).

{{< tabpane lang="yaml" right=true >}}
{{< tab header="Strategic merge patch" >}}
cluster:
  network:
    podSubnets:
      - 192.168.0.0/16
    serviceSubnets:
      - 192.0.0.0/12
    cni:
      name: none
{{< /tab >}}
{{< tab header="JSON patch" >}}
- op: replace
  path: /cluster/network/podSubnets
  value:
    - 192.168.0.0/16
- op: replace
  path: /cluster/network/serviceSubnets
  value:
    - 192.0.0.0/12
- op: add
  path: /cluster/network/cni
  value:
    name: none
{{< /tab >}}
{{< /tabpane >}}

Patched machine configuration:

```yaml
cluster:
  network:
    dnsDomain: cluster.local
    podSubnets:
      - 192.168.0.0/16
    serviceSubnets:
      - 192.0.0.0/12
    cni:
      name: none
```

### Kubelet

Base machine configuration:

```yaml
# ...
machine:
  kubelet: {}
```

The goal is to set the `kubelet` node IP to come from the subnet `192.168.10.0/24`.

{{< tabpane lang="yaml" right=true >}}
{{< tab header="Strategic merge patch" >}}
machine:
  kubelet:
    nodeIP:
      validSubnets:
        - 192.168.10.0/24
{{< /tab >}}
{{< tab header="JSON patch" >}}
- op: add
  path: /machine/kubelet/nodeIP
  value:
    validSubnets:
      - 192.168.10.0/24
{{< /tab >}}
{{< /tabpane >}}

Patched machine configuration:

```yaml
machine:
  kubelet:
    nodeIP:
      validSubnets:
        - 192.168.10.0/24
```

### Admission Control: Pod Security Policy

Base machine configuration:

```yaml
cluster:
  apiServer:
    admissionControl:
      - name: PodSecurity
        configuration:
          apiVersion: pod-security.admission.config.k8s.io/v1alpha1
          defaults:
            audit: restricted
            audit-version: latest
            enforce: baseline
            enforce-version: latest
            warn: restricted
            warn-version: latest
          exemptions:
            namespaces:
              - kube-system
            runtimeClasses: []
            usernames: []
          kind: PodSecurityConfiguration
```

The goal is to add an exemption for the namespace `rook-ceph`.

{{< tabpane lang="yaml" right=true >}}
{{< tab header="Strategic merge patch" >}}
cluster:
  apiServer:
    admissionControl:
      - name: PodSecurity
        configuration:
          exemptions:
            namespaces:
              - rook-ceph
{{< /tab >}}
{{< tab header="JSON patch" >}}
- op: add
  path: /cluster/apiServer/admissionControl/0/configuration/exemptions/namespaces/-
  value: rook-ceph
{{< /tab >}}
{{< /tabpane >}}

Patched machine configuration:

```yaml
cluster:
  apiServer:
    admissionControl:
      - name: PodSecurity
        configuration:
          apiVersion: pod-security.admission.config.k8s.io/v1alpha1
          defaults:
            audit: restricted
            audit-version: latest
            enforce: baseline
            enforce-version: latest
            warn: restricted
            warn-version: latest
          exemptions:
            namespaces:
              - kube-system
              - rook-ceph
            runtimeClasses: []
            usernames: []
          kind: PodSecurityConfiguration
```

## Configuration Patching with `talosctl` CLI

Several `talosctl` commands accept config patches as command-line flags.
Config patches might be passed either as an inline value or as a reference to a file with `@file.patch` syntax:

```shell
talosctl ... --patch '[{"op": "add", "path": "/machine/network/hostname", "value": "worker1"}]' --patch @file.patch
```

If multiple config patches are specified, they are applied in the order of appearance.
The format of the patch (JSON patch or strategic merge patch) is detected automatically.

Talos machine configuration can be patched at the moment of generation with `talosctl gen config`:

```shell
talosctl gen config test-cluster https://172.20.0.1:6443 \
  --config-patch '[{"op": "add", "path": "/machine/certSANs", "value": ["10.0.0.10"]}]' \
  --config-patch @all.yaml \
  --config-patch-control-plane @cp.yaml \
  --config-patch-worker @worker.yaml
```

Generated machine configuration can also be patched after the fact with `talosctl machineconfig patch`

```shell
talosctl machineconfig patch worker.yaml --patch @patch.yaml -o worker1.yaml
```

Machine configuration on the running Talos node can be patched with `talosctl patch`:

```shell
talosctl patch mc --nodes 172.20.0.2 --patch @patch.yaml
```
