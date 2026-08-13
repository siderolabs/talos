---
description: StaticHostConfig is a config document to set /etc/hosts entries.
title: StaticHostConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: StaticHostConfig
name: 10.5.0.2 # IP address (IPv4 or IPv6) to map the hostnames to.
# List of hostnames to map to the IP address.
hostnames:
    - my-server
    - my-server.example.org
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |IP address (IPv4 or IPv6) to map the hostnames to.  | |
|`hostnames` |[]string |List of hostnames to map to the IP address.  | |






