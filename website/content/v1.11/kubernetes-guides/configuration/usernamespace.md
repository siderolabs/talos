---
title: "User Namespaces"
description: "Guide on how to configure Talos Cluster to support User Namespaces"
---

User Namespaces are a feature of the Linux kernel that allows unprivileged users to have their own range of UIDs and GIDs, without needing to be root.

Refer to the [official documentation](https://kubernetes.io/docs/concepts/workloads/pods/user-namespaces/) for more information on Usernamespaces.

## Enabling Usernamespaces

To enable User Namespaces in Talos, you need to add the following configuration to Talos machine configuration:

```yaml
---
cluster:
  apiServer:
    extraArgs:
      feature-gates: UserNamespacesSupport=true,UserNamespacesPodSecurityStandards=true
machine:
  sysctls:
    user.max_user_namespaces: "11255"
  kubelet:
    extraConfig:
      featureGates:
        UserNamespacesSupport: true
        UserNamespacesPodSecurityStandards: true
```

After applying the configuration, refer to the [official documentation](https://kubernetes.io/docs/tasks/configure-pod-container/user-namespaces/) to configure workloads to use User Namespaces.
