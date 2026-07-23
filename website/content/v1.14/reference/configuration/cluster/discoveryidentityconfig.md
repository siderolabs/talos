---
description: DiscoveryIdentityConfig is a config document to configure the cluster
    identity used by the discovery service.
title: DiscoveryIdentityConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: DiscoveryIdentityConfig
clusterID: cluster-id-base64-encoded-32-bytes # Globally unique identifier for this cluster (base64 encoded random 32 bytes).
clusterSecret: cluster-secret-base64-encoded-32-bytes # Shared secret of cluster (base64 encoded random 32 bytes).
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`clusterID` |string |Globally unique identifier for this cluster (base64 encoded random 32 bytes).  | |
|`clusterSecret` |string |Shared secret of cluster (base64 encoded random 32 bytes).<br>This secret is shared among cluster members but should never be sent over the network.  | |






