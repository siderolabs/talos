---
title: "Custom Certificate Authorities"
description: "How to supply custom certificate authorities"
aliases:
  - ../../guides/configuring-certificate-authorities
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
