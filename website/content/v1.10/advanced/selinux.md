---
title: "SELinux"
description: "SELinux security module support (experimental)."
---

Talos Linux 1.10 introduces initial SELinux support.
Talos currently contains a basic SELinux policy that is designed to protect the OS from workloads, including privileged pods.
The policy denies access to machine configuration, prevents debuggers from attaching to system processes, however it is out of its scope to secure the Kubernetes components themselves.

## Configuration

SELinux is enabled by default in Talos 1.10 images.
The default mode is permissive, as currently some CNI and CSI solutions as well as extensions are incompatible with it.
For now, enforcing mode has only been tested with the Flannel CNI we ship by default.
These missing parts are being worked on to make SELinux available for more use cases.

### Mode of operation

You can query the SELinux state with:

```shell
$ talosctl --nodes <IP> get SecurityState
NODE         NAMESPACE   TYPE            ID              VERSION   SECUREBOOT   UKISIGNINGKEYFINGERPRINT   PCRSIGNINGKEYFINGERPRINT   SELINUXSTATE
172.20.0.2   runtime     SecurityState   securitystate   1         false                                                              enabled, permissive
```

> Please note that SELinux is still in an experimental state in Talos Linux.
> Extensions currently do not support enforcing mode, which is a known missing feature being worked on.
> Expect some CNI and CSI plugins to not work in enforcing mode.
> Please report the issues you encounter with different configurations to help cover various usage scenarios.
> Enforcing mode should only be enabled on new installs as of version 1.10, since the upgrade path for enabling SELinux is still being worked on.

As for version 1.10, SELinux runs in permissive mode by default, which does not offer any extra protection, but allows to log denials.
SELinux can be put in enforcing mode (to actually prevent access when it is not authorized by the policy) by adding `enforcing=1` to the kernel cmdline.
This is most commonly done via the configuration in the Image Factory.

## Obtaining and processing denial logs

If SELinux has blocked some event from happening, it will log it to the audit log.
If the mode is permissive, the only implication of that would be a denial message, so permissive mode is useful for prototyping the policy.
You can check the logs with:

`talosctl --nodes <IP> logs auditd > audit.log`

You can get more insights on SELinux policy inner workings in the corresponding section of the [Developing Talos]({{< relref "./developing-talos/#selinux-policy-debugging-and-development" >}}) page.
