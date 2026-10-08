---
description: SELinuxPolicyConfig is a SELinux policy module document.
title: SELinuxPolicyConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: SELinuxPolicyConfig
name: hostmon # Name of the policy module.
content: | # Policy module in CIL, compiled with the Talos policy and loaded without a reboot.
    (type pod_hostmon_t)
    (call pod_hostmon_domain (pod_hostmon_t))
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of the policy module.  | |
|`content` |string |Policy module in CIL, compiled with the Talos policy and loaded without a reboot.<br>A module declares a type and calls a macro of the base policy, whose sources are in `/usr/share/selinux/talos`:<br>`pod_domain` gives the rights of `pod_t`, `pod_privileged_domain` those of `pod_privileged_t`, and<br>`pod_hostmon_domain` those of a process monitor, which reads `/proc` and the files of every pod and writes none.<br>Whatever a module grants, a workload domain never reads STATE, connects to machined or ptraces a host service:<br>`secilc` rejects such a module, which is left out while the other modules are loaded.<br>A workload selects the type with `securityContext.seLinuxOptions.type`, which the CRI applies when the<br>`SecurityProfileConfig` document sets `selinuxCategories`: load the module before the workload starts, and<br>remove the workload before the module. A privileged container cannot select a type, it runs as<br>`pod_privileged_t`, which `spc_t` is an alias of.  | |






