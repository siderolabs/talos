---
title: "SideroLink"
description: "Point-to-point management overlay Wireguard network."
---

SideroLink provides a secure point-to-point management overlay network for Talos clusters.
Each Talos machine configured to use SideroLink will establish a secure Wireguard connection to the SideroLink API server.
SideroLink provides overlay network using ULA IPv6 addresses allowing to manage Talos Linux machines even if direct access to machine IP addresses is not possible.
SideroLink is a foundation building block of [Sidero Omni](https://www.siderolabs.com/platform/saas-for-kubernetes/).

## Configuration

SideroLink is configured by providing the SideroLink API server address, either via kernel command line argument `siderolink.api` or as a [config document]({{< relref "../../reference/configuration/siderolink/siderolinkconfig" >}}).

SideroLink API URL: `https://siderolink.api/?jointoken=token&grpc_tunnel=true`.
If URL scheme is `grpc://`, the connection will be established without TLS, otherwise, the connection will be established with TLS.
If specified, join token `token` will be sent to the SideroLink server.
If `grpc_tunnel` is set to `true`, the Wireguard traffic will be tunneled over the same SideroLink API gRPC connection instead of using plain UDP.

## Connection Flow

1. Talos Linux creates an ephemeral Wireguard key.
2. Talos Linux establishes a gRPC connection to the SideroLink API server, sends its own Wireguard public key, join token and other connection settings.
3. If the join token is valid, the SideroLink API server sends back the Wireguard public key of the SideroLink API server, and two overlay IPv6 addresses: machine address and SideroLink server address.
4. Talos Linux configured Wireguard interface with the received settings.
5. Talos Linux monitors status of the Wireguard connection and re-establishes the connection if needed.

## Operations with SideroLink

When SideroLink is configured, Talos maintenance mode API listens only on the SideroLink network.
Maintenance mode API over SideroLink allows operations which are not generally available over the public network: getting Talos version, getting sensitive resources, etc.

Talos Linux always provides Talos API over SideroLink, and automatically allows access over SideroLink even if the [Ingress Firewall]({{< relref "./ingress-firewall" >}}) is enabled.
Wireguard connections should be still allowed by the Ingress Firewall.

SideroLink only allows point-to-point connections between Talos machines and the SideroLink management server, two Talos machines cannot communicate directly over SideroLink.
