---
description: KubeServiceAccountConfig configures Kubernetes service accounts.
title: KubeServiceAccountConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: KubeServiceAccountConfig
# The service account issuer configuration.
issuer:
    privateKey: '--- EXAMPLE PRIVATE KEY ---' # The key which is used to sign the service account tokens.
    issuerURL: https://my-control-plane:6443 # The issuer URL which is used to sign the service account tokens.
# The additional service accounts which are accepted by the Kubernetes API server.
accepted:
    # The list of public keys which are used to verify the service account tokens.
    publicKeys:
        - '--- EXAMPLE PUBLIC KEY ---'
    # The additional service account issuers which are accepted by the Kubernetes API server.
    issuers:
        - https://another-control-plane:6443
    # The list of API audiences for which the service account tokens are accepted by the Kubernetes API server.
    audiences:
        - https://another-control-plane:6443
        - https://my-control-plane:6443
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`issuer` |<a href="#KubeServiceAccountConfig.issuer">IssuerServiceAccountConfig</a> |The service account issuer configuration.<br><br>This configures how the service accounts are issued in Kubernetes.  | |
|`accepted` |<a href="#KubeServiceAccountConfig.accepted">AcceptedServiceAccountConfig</a> |The additional service accounts which are accepted by the Kubernetes API server.<br><br>This might be used for service account rotation, or for accepting service accounts from other clusters,<br>or for accepting service accounts from other issuers.  | |




## issuer {#KubeServiceAccountConfig.issuer}

IssuerServiceAccountConfig configures the service account issuer.





| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`privateKey` |string |The key which is used to sign the service account tokens.<br><br>This key is used to sign the service account tokens, and it is used by the Kubernetes API server to verify the service account tokens.<br>The key must be a valid PEM encoded RSA or ECDSA private key.  | |
|`issuerURL` |URL |The issuer URL which is used to sign the service account tokens.<br><br>This URL is used to sign the service account tokens, and it is used by the Kubernetes API server to verify the service account tokens.  | |






## accepted {#KubeServiceAccountConfig.accepted}

AcceptedServiceAccountConfig configures the accepted service accounts.





| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`publicKeys` |[]string |The list of public keys which are used to verify the service account tokens.<br><br>These keys are used by the Kubernetes API server to verify the service account tokens.<br>The keys must be valid PEM encoded RSA or ECDSA public keys.  | |
|`issuers` |[]URL |The additional service account issuers which are accepted by the Kubernetes API server.<br><br>This might be used for service account rotation, or for accepting service accounts from other clusters,<br>or for accepting service accounts from other issuers.  | |
|`audiences` |[]string |The list of API audiences for which the service account tokens are accepted by the Kubernetes API server.<br><br>If this field is not set, the default is to set to the issuer URL of the service account issuer.  | |








