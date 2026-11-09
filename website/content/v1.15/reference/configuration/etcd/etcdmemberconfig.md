---
description: |
    EtcdMemberConfig configures the etcd member running on the machine.
    The presence of this document enables the etcd member on the machine.
    This document requires the `EtcdCAConfig` document to be present as well.

    This document should only be present on the controlplane machines.
title: EtcdMemberConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: EtcdMemberConfig
image: registry.k8s.io/etcd:v3.7.3 # The container image used to run etcd.
# Extra arguments to supply to etcd.
extraArgs:
    election-timeout: "5000"
# The `advertisedSubnets` field configures the networks to pick etcd advertised IP from.
advertisedSubnets:
    - 10.0.0.0/8
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`image` |string |The container image used to run etcd.<br><br>If not set, the default etcd image for the Talos version is used. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
image: registry.k8s.io/etcd:v3.7.3
{{< /highlight >}}</details> | |
|`extraArgs` |Args |Extra arguments to supply to etcd.<br><br>Note that the following args are not allowed:<br><br>- `name`<br>- `data-dir`<br>- `initial-cluster-state`<br>- `listen-peer-urls`<br>- `listen-client-urls`<br>- `cert-file`<br>- `key-file`<br>- `trusted-ca-file`<br>- `peer-client-cert-auth`<br>- `peer-cert-file`<br>- `peer-trusted-ca-file`<br>- `peer-key-file`  | |
|`advertisedSubnets` |[]string |The `advertisedSubnets` field configures the networks to pick etcd advertised IP from.<br><br>IPs can be excluded from the list by using negative match with `!`, e.g `!10.0.0.0/8`.<br>Negative subnet matches should be specified last to filter out IPs picked by positive matches.<br>If not specified, advertised IP is selected as the first routable address of the node. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
advertisedSubnets:
    - 10.0.0.0/8
{{< /highlight >}}</details> | |
|`listenSubnets` |[]string |The `listenSubnets` field configures the networks for the etcd to listen for peer and client connections.<br><br>If `listenSubnets` is not set, but `advertisedSubnets` is set, `listenSubnets` defaults to<br>`advertisedSubnets`.<br><br>If neither `advertisedSubnets` nor `listenSubnets` is set, `listenSubnets` defaults to listen on all addresses.<br><br>IPs can be excluded from the list by using negative match with `!`, e.g `!10.0.0.0/8`.<br>Negative subnet matches should be specified last to filter out IPs picked by positive matches.  | |






