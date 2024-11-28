---
title: "OCI Base Runtime Specification"
description: "Adjusting OCI base runtime specification for CRI containers."
---

Every container initiated by the Container Runtime Interface (CRI) adheres to the [OCI runtime specification](https://github.com/opencontainers/runtime-spec/blob/main/spec.md).
While certain aspects of this specification can be modified through Kubernetes pod and container configurations, others remain fixed.

Talos Linux provides the capability to adjust the OCI base runtime specification for all containers managed by the CRI.
However, it is important to note that the Kubernetes/CRI plugin may still override some settings, meaning changes to the base runtime specification are not always guaranteed to take effect.

## Getting Current OCI Base Runtime Specification

To get the current OCI base runtime specification, you can use the following command (`yq -P .` is used to pretty-print the output):

```bash
$ talosctl read /etc/cri/conf.d/base-spec.json | yq -P .
ociVersion: 1.2.0
process:
  user:
    uid: 0
    gid: 0
  cwd: /
  capabilities:
    bounding:
      - CAP_CHOWN
...
```

The output might depend on a specific Talos (`containerd`) version.

## Adjusting OCI Base Runtime Specification

To adjust the OCI base runtime specification, the following machine configuration patch can be used:

```yaml
machine:
  baseRuntimeSpecOverrides:
    process:
      rlimits:
        - type: RLIMIT_NOFILE
          hard: 1024
          soft: 1024
```

In this example, the number of open files is adjusted to be 1024 for all containers (OCI default is unset, so it inherits the Talos default of 1048576 open files).
The contents of the `baseRuntimeSpecOverrides` field are merged with the current base runtime specification, so only the fields that need to be adjusted should be included.

This configuration change will be applied with a machine reboot, and OCI base runtime specification will only affect new containers created after the change on the node.
