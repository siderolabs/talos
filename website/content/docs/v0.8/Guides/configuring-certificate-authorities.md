---
title: "Configuring Certificate Authorities"
description: ""
---

## Appending the Certificate Authority

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
