---
title: "Knowledge Base"
weight: 1999
description: "Recipes for common configuration tasks with Talos Linux."
---

## Disabling `GracefulNodeShutdown` on a node

Talos Linux enables [Graceful Node Shutdown](https://kubernetes.io/docs/concepts/architecture/nodes/#graceful-node-shutdown) Kubernetes feature by default.

If this feature should be disabled, modify the `kubelet` part of the machine configuration with:

```yaml
machine:
  kubelet:
    extraArgs:
      feature-gates: GracefulNodeShutdown=false
    extraConfig:
      shutdownGracePeriod: 0s
      shutdownGracePeriodCriticalPods: 0s
```
