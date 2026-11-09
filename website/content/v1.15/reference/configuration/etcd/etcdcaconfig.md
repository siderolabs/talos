---
description: |
    EtcdCAConfig configures etcd PKI (certificate authority).
    The etcd CA is used to issue the certificates for the etcd members (server and peer),
    and the etcd clients (e.g. kube-apiserver, Talos itself).

    The accepted CAs are trusted by the etcd members (for client and peer connections),
    and by the etcd clients (for server connections), in addition to the issuing CA.

    This document should only be present on the controlplane machines.
title: EtcdCAConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: EtcdCAConfig
# The issuing certificate authority for etcd.
issuingCA:
    cert: '--- EXAMPLE CERTIFICATE ---'
    key: '--- EXAMPLE KEY ---'
# The list of additional accepted CA certificates for etcd.
acceptedCAs:
    - '--- EXAMPLE CA ---'
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`issuingCA` |CertificateAndKey |The issuing certificate authority for etcd.<br><br>The value contains a private key and a certificate, PEM encoded.  | |
|`acceptedCAs` |[]string |The list of additional accepted CA certificates for etcd.<br><br>The value should be a PEM encoded certificate.<br>The issuing CA certificate is automatically added to the list of accepted CAs.  | |






