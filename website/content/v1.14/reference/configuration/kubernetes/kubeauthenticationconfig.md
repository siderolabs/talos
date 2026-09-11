---
description: KubeAuthenticationConfig configures kube-apiserver authentication.
title: KubeAuthenticationConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: KubeAuthenticationConfig
# Kubernetes API server [authentication](https://kubernetes.io/docs/reference/access-authn-authz/authentication/) configuration.
configuration:
    anonymous:
        conditions:
            - path: /livez
            - path: /readyz
            - path: /healthz
        enabled: true
    jwt: []
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`configuration` |Unstructured |Kubernetes API server [authentication](https://kubernetes.io/docs/reference/access-authn-authz/authentication/) configuration.<br>The value is the literal Kubernetes authentication configuration.<br><br>This document replaces the legacy kube-apiserver authentication flags, which are rejected in<br>`KubeAPIServerConfig` `extraArgs`: kube-apiserver refuses any `--oidc-*` flag whenever<br>`--authentication-config` is set, and Talos always sets it.<br><br>Flag equivalents:<br>`--anonymous-auth` -> `anonymous.enabled`,<br>`--oidc-issuer-url` -> `jwt[].issuer.url`,<br>`--oidc-client-id` -> `jwt[].issuer.audiences`,<br>`--oidc-ca-file` -> `jwt[].issuer.certificateAuthority`,<br>`--oidc-username-claim` -> `jwt[].claimMappings.username.claim`,<br>`--oidc-username-prefix` -> `jwt[].claimMappings.username.prefix`,<br>`--oidc-groups-claim` -> `jwt[].claimMappings.groups.claim`,<br>`--oidc-groups-prefix` -> `jwt[].claimMappings.groups.prefix`,<br>`--oidc-required-claim` -> `jwt[].claimValidationRules[]`.<br><br>`--oidc-signing-algs` has no equivalent and cannot be configured: with structured authentication<br>configuration kube-apiserver accepts every RFC 7518 asymmetric algorithm<br>(RS256, RS384, RS512, ES256, ES384, ES512, PS256, PS384, PS512).  | |






