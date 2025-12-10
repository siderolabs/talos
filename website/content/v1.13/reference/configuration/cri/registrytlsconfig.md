---
description: RegistryTLSConfig configures TLS for a registry endpoint.
title: RegistryTLSConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: RegistryTLSConfig
name: my-private-registry.local:5000 # Registry endpoint to apply the TLS configuration to.
ca: |- # CA registry certificate to add the list of trusted certificates.
    -----BEGIN CERTIFICATE-----
    MIID...IDAQAB
    -----END CERTIFICATE-----

# # Enable mutual TLS authentication with the registry.
# clientIdentity:
#     cert: |-
#         -----BEGIN CERTIFICATE-----
#         MIID...IDAQAB
#         -----END CERTIFICATE-----
#     key: |-
#         -----BEGIN PRIVATE KEY-----
#         MIIE...AB
#         -----END PRIVATE KEY-----
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Registry endpoint to apply the TLS configuration to.<br><br>Registry endpoint is the hostname part of the endpoint URL,<br>e.g. 'my-mirror.local:5000' for 'https://my-mirror.local:5000/v2/'.<br><br>The TLS configuration makes sense only for HTTPS endpoints.<br>The TLS configuration will apply to all image pulls for this<br>registry endpoint, by Talos or any Kubernetes workloads.  | |
|`clientIdentity` |CertificateAndKey |Enable mutual TLS authentication with the registry.<br>Client certificate and key should be PEM-encoded. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
clientIdentity:
    cert: |-
        -----BEGIN CERTIFICATE-----
        MIID...IDAQAB
        -----END CERTIFICATE-----
    key: |-
        -----BEGIN PRIVATE KEY-----
        MIIE...AB
        -----END PRIVATE KEY-----
{{< /highlight >}}</details> | |
|`ca` |string |CA registry certificate to add the list of trusted certificates.<br>Certificate should be PEM-encoded.  | |
|`insecureSkipVerify` |bool |Skip TLS server certificate verification (not recommended).  | |






