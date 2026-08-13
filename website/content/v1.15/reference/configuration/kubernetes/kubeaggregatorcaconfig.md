---
description: KubeAggregatorCAConfig configures Kubernetes API aggregator accepted
    CAs.
title: KubeAggregatorCAConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: KubeAggregatorCAConfig
# The currently active issuing certificate authority for the Kubernetes API aggregator flow.
issuingCA:
    cert: '--- EXAMPLE CERTIFICATE ---'
    key: '--- EXAMPLE KEY ---'
# The list of accepted CA certificates for the Kubernetes API server aggregator flow.
acceptedCAs:
    - '--- EXAMPLE AGGREGATOR CA ---'
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`issuingCA` |CertificateAndKey |The currently active issuing certificate authority for the Kubernetes API aggregator flow.<br><br>This field should only be set for the controlplane machines.<br>The value contains a private key and a certificate, PEM encoded.  | |
|`acceptedCAs` |[]string |The list of accepted CA certificates for the Kubernetes API server aggregator flow.<br><br>This field should only be set for the controlplane machines.<br>The value should be a PEM encoded certificate.<br>The issuing CA certificate is automatically added to the list of accepted CAs.  | |






