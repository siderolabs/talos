---
title: "Configuring Corporate Proxies"
description: ""
---

## Appending the Certificate Authority of MITM Proxies

Put into each machine the PEM encoded certificate:

```yaml
machine:
  ...
  files:
    - content: |
        -----BEGIN CERTIFICATE-----
        ...
        -----END CERTIFICATE-----
      permissions: 0644
      path: /etc/ssl/certs/ca-certificates
      op: append
```

## Configuring a Machine to Use the Proxy

To make use of a proxy:

```yaml
machine:
  env:
    http_proxy: <http proxy>
    https_proxy: <https proxy>
    no_proxy: <no proxy>
```

Additionally, configure the DNS `nameservers`, and NTP `servers`:

```yaml
machine:
  env:
  ...
  time:
    servers:
      - <server 1>
      - <server ...>
      - <server n>
  ...
  network:
    nameservers:
      - <ip 1>
      - <ip ...>
      - <ip n>
```
