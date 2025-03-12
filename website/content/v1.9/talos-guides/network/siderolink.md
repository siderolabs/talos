---
title: "SideroLink"
description: "Point-to-point management overlay Wireguard network."
---

SideroLink offers a secure point-to-point management overlay network for Talos clusters using Wireguard.
Each Talos machine configured with SideroLink establishes a secure Wireguard connection to the SideroLink API server.
This overlay network utilizes ULA IPv6 addresses, enabling the management of Talos Linux machines even when direct access to their IP addresses is not feasible.
SideroLink is a fundamental component of [Sidero Omni](https://www.siderolabs.com/platform/saas-for-kubernetes/).

## Configuration

To configure SideroLink, provide the SideroLink API server address either via the kernel command line argument `siderolink.api` or as a [config document]({{< relref "../../reference/configuration/siderolink/siderolinkconfig" >}}).

The SideroLink API URL format is: `https://siderolink.api/?jointoken=token&grpc_tunnel=true`.

- If the URL scheme is `grpc://`, the connection will be established without TLS; otherwise, it will use TLS.
- The join token `token`, if specified, will be sent to the SideroLink server.
- Setting `grpc_tunnel` to `true` sends a hint to tunnel Wireguard traffic over the same SideroLink API gRPC connection instead of using plain UDP.
  This is useful in environments where UDP traffic is restricted but adds significant overhead to SideroLink communication, enable this only if necessary.
  Note that the SideroLink API server might ignore this hint, and the connection might use gRPC tunneling regardless of the setting.

## Connection Flow

1. Talos Linux generates an ephemeral Wireguard key.
2. Talos Linux establishes a gRPC connection to the SideroLink API server, sending its Wireguard public key, join token, and other connection settings.
3. If the join token is valid, the SideroLink API server responds with its Wireguard public key and two overlay IPv6 addresses: one for the machine and one for the SideroLink server.
4. Talos Linux configures the Wireguard interface with the received settings.
5. Talos Linux monitors the Wireguard connection status and re-establishes the connection if necessary.

## Operations with SideroLink

When SideroLink is configured, the Talos maintenance mode API listens exclusively on the SideroLink network.
This allows operations not generally available over the public network, such as retrieving the Talos version and accessing sensitive resources.

Talos Linux always provides the Talos API over SideroLink and automatically permits access over SideroLink even if the [Ingress Firewall]({{< relref "./ingress-firewall" >}}) is enabled.
However, Wireguard connections must still be allowed by the Ingress Firewall.

SideroLink only supports point-to-point connections between Talos machines and the SideroLink management server; direct communication between two Talos machines over SideroLink is not possible.
