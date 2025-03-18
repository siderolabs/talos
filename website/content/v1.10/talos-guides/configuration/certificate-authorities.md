---
title: "Custom Certificate Authorities"
description: "How to supply custom certificate authorities"
aliases:
  - ../../guides/configuring-certificate-authorities
---

## Appending the Certificate Authority

Append additional certificate authorities to the system's trusted certificate store by [patching]({{< relref "./patching" >}}) the machine configuration with the following
[document]({{< relref "../../reference/configuration/security/trustedrootsconfig" >}}):

```yaml
apiVersion: v1alpha1
kind: TrustedRootsConfig
name: custom-ca
certificates: |-
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----
```

Multiple documents can be appended, and multiple CA certificates might be present in each configuration document.

This configuration can be also applied in maintenance mode.

Please note that if the `STATE` partition is encrypted, the CA certificates will be only be loaded after the partition is unlocked.
So the encryption method should allow unlocking the partition without the need for a CA certificate.
