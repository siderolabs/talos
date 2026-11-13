---
description: KubeAPIServerCAConfig configures Kubernetes API server CA.
title: KubeAPIServerCAConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: KubeAPIServerCAConfig
# The currently active issuing certificate authority for the Kubernetes API server.
issuingCA:
    cert: '--- EXAMPLE CERTIFICATE ---'
    key: '--- EXAMPLE KEY ---'
# The list of accepted CA certificates for the Kubernetes API server.
acceptedCAs:
    - '--- EXAMPLE CA ---'
rootCABundle: '--- EXAMPLE CA BUNDLE ---' # The CA bundle published to the pods as the `kube-root-ca.crt` ConfigMap.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`issuingCA` |CertificateAndKey |The currently active issuing certificate authority for the Kubernetes API server.<br><br>This field should only be set for the controlplane machines.<br>The value contains a private key and a certificate, PEM encoded.  | |
|`acceptedCAs` |[]string |The list of accepted CA certificates for the Kubernetes API server.<br><br>The value should be a PEM encoded certificate.<br>The issuing CA certificate is automatically added to the list of accepted CAs.  | |
|`rootCABundle` |string |The CA bundle published to the pods as the `kube-root-ca.crt` ConfigMap.<br><br>By default, the issuing CA certificate is published.<br>When the issuing CA is an intermediate CA, the bundle can contain its parent chain,<br>so that clients which require a self-signed trust anchor (e.g. OpenSSL) can verify the API server.<br>The bundle is not trusted to authenticate clients, and it is reset when the issuing CA is rotated.<br><br>Publishing a parent CA expands the trust of the workloads to any certificate issued under it:<br>a dedicated self-signed CA per cluster (the default) is recommended.<br><br>This field should only be set for the controlplane machines.<br>The value should be PEM encoded certificates which allow to verify the issuing CA.  | |






