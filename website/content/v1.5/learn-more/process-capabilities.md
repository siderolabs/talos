---
title: "Process Capabilities"
weight: 105
description: "Understand the Linux process capabilities restrictions with Talos Linux."
---

Linux defines a set of [process capabilities](https://man7.org/linux/man-pages/man7/capabilities.7.html) that can be used to fine-tune the process permissions.

Talos Linux for security reasons restricts any process from gaining the following capabilities:

* `CAP_SYS_MODULE` (loading kernel modules)
* `CAP_SYS_BOOT` (rebooting the system)

This means that any process including privileged Kubernetes pods will not be able to get these capabilities.

If you see the following error on starting a pod, make sure it doesn't have any of the capabilities listed above in the spec:

```text
Error: failed to create containerd task: failed to create shim task: OCI runtime create failed: runc create failed: unable to start container process: unable to apply caps: operation not permitted: unknown
```

> Note: even with `CAP_SYS_MODULE` capability, Linux kernel module loading is restricted by requiring a valid signature.
> Talos Linux creates a throw away signing key during kernel build, so it's not possible to build/sign a kernel module for Talos Linux outside of the build process.
