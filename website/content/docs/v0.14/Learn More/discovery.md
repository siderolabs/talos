---
title: "Discovery"
weight: 11
---

We maintain a public discovery service whereby members of your cluster can use a common and unique key to coordinate the most basic connection information (i.e. the set of possible "endpoints", or IP:port pairs).
We call this data "affiliate data."

> Note: If KubeSpan is enabled the data has the addition of the WireGuard public key.

Before sending data to the discovery service, Talos will encrypt the affiliate data with AES-GCM encryption and separately encrypt endpoints with AES in ECB mode so that endpoints coming from different sources can be deduplicated server-side.
Each node submits it's data encrypted plus it submits the endpoints it sees from other peers to the discovery service
The discovery service aggregates the data, deduplicates the endpoints, and sends updates to each connected peer.
Each peer receives information back about other affiliates from the discovery service, decrypts it and uses it to drive KubeSpan and cluster discovery.
Moreover, the discovery service has no persistence.
Data is stored in memory only with a TTL set by the clients (i.e. Talos).
The cluster ID is used as a key to select the affiliates (so that different clusters see different affiliates).

To summarize, the discovery service knows the client version, cluster ID, the number of affiliates, some encrypted data for each affiliate, and a list of encrypted endpoints.
