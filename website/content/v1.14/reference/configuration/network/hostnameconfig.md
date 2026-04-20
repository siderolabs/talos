---
description: 'HostnameConfig is a config document to configure the hostname: either a static hostname or an automatically generated hostname.'
title: HostnameConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: HostnameConfig
hostname: worker-33 # A static hostname to set for the machine.
{{< /highlight >}}

{{< highlight yaml >}}
apiVersion: v1alpha1
kind: HostnameConfig
auto: stable # A method to automatically generate a hostname for the machine.

# # A static hostname to set for the machine.
# hostname: controlplane1
# hostname: controlplane1.example.org
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`auto` |AutoHostnameKind |A method to automatically generate a hostname for the machine.<br><br>There are two methods available:<br>  - `stable` - generates a stable hostname based on machine identity<br>  - `off` - disables automatic hostname generation, Talos will wait for an external source to provide a hostname (DHCP, cloud-init, etc).<br><br>Automatic hostnames have the lowest priority over any other hostname sources: DHCP, cloud-init, etc.<br>Conflicts with `hostname` field.  |`stable`<br />`off`<br /> |
|`hostname` |string |A static hostname to set for the machine.<br><br>This hostname has the highest priority over any other hostname sources: DHCP, cloud-init, etc.<br>Conflicts with `auto` field. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
hostname: controlplane1
{{< /highlight >}}{{< highlight yaml >}}
hostname: controlplane1.example.org
{{< /highlight >}}</details> | |






