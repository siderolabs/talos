---
description: SysctlConfig configures Linux kernel sysctl values.
title: SysctlConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: SysctlConfig
# Used to configure the machine's sysctls (kernel parameters under `/proc/sys`).
params:
    fs.inotify.max_user_watches: "12288"
    kernel.domainname: talos.dev
    net.ipv4.ip_forward: "0"
    net/ipv6/conf/eth0.100/disable_ipv6: "1"
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`params` |map[string]string |Used to configure the machine's sysctls (kernel parameters under `/proc/sys`).<br>Values from this document are merged with the deprecated v1alpha1 machine.sysctls values (if set),<br>with this document taking precedence on key conflicts. <details><summary>Show example(s)</summary>SysctlConfig usage example.:{{< highlight yaml >}}
params:
    fs.inotify.max_user_watches: "12288"
    kernel.domainname: talos.dev
    net.ipv4.ip_forward: "0"
    net/ipv6/conf/eth0.100/disable_ipv6: "1"
{{< /highlight >}}</details> | |






