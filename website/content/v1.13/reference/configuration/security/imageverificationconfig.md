---
description: ImageVerificationConfig configures image signature verification policy.
title: ImageVerificationConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: ImageVerificationConfig
# List of verification rules.
rules:
    - image: registry.k8s.io/* # Image reference pattern to match for this rule.
      verify: true # Whether or not to verify matching references.
      # Keyless verifier configuration to use for this rule.
      keyless:
        issuer: https://accounts.google.com # OIDC issuer URL for keyless verification.
        subject: krel-trust@k8s-releng-prod.iam.gserviceaccount.com # Expected subject for keyless verification.

        # # Regex pattern for subject matching.
        # subjectRegex: .*@example\.com
    - image: my-registry/* # Image reference pattern to match for this rule.
      verify: true # Whether or not to verify matching references.
      # Public key verifier configuration to use for this rule.
      publicKey:
        certificate: |- # A public certificate in PEM format accepted for image signature verification.
            -----BEGIN CERTIFICATE-----
            MII--Sample Value--
            -----END CERTIFICATE-----
    - image: '**' # Image reference pattern to match for this rule.
      verify: true # Whether or not to verify matching references.
      # Keyless verifier configuration to use for this rule.
      keyless:
        issuer: https://token.actions.githubusercontent.com # OIDC issuer URL for keyless verification.
        subjectRegex: https://github.com/myorg/.* # Regex pattern for subject matching.
        rekorURL: https://rekor.sigstore.dev # Rekor transparency log URL (optional, defaults to "https://rekor.sigstore.dev").
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`rules` |<a href="#ImageVerificationConfig.rules.">[]ImageVerificationRuleV1Alpha1</a> |List of verification rules.<br>Rules are evaluated in order; first matching rule applies.  | |




## rules[] {#ImageVerificationConfig.rules.}

ImageVerificationRuleV1Alpha1 defines a verification rule.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`image` |string |Image reference pattern to match for this rule.<br>Supports glob patterns. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
image: docker.io/library/nginx
{{< /highlight >}}{{< highlight yaml >}}
image: registry.k8s.io/*
{{< /highlight >}}</details> | |
|`verify` |bool |Whether or not to verify matching references.  | |
|`keyless` |<a href="#ImageVerificationConfig.rules..keyless">ImageKeylessVerifierV1Alpha1</a> |Keyless verifier configuration to use for this rule.  | |
|`publicKey` |<a href="#ImageVerificationConfig.rules..publicKey">ImagePublicKeyVerifierV1Alpha1</a> |Public key verifier configuration to use for this rule.  | |




### keyless {#ImageVerificationConfig.rules..keyless}

ImageKeylessVerifierV1Alpha1 configures a signature verification provider using Cosign keyless verification.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`issuer` |string |OIDC issuer URL for keyless verification. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
issuer: https://accounts.google.com
{{< /highlight >}}{{< highlight yaml >}}
issuer: https://token.actions.githubusercontent.com
{{< /highlight >}}</details> | |
|`subject` |string |Expected subject for keyless verification.<br><br>This is the identity (email, URI) that signed the image.  | |
|`subjectRegex` |string |Regex pattern for subject matching.<br><br>Use this instead of subject for flexible matching. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
subjectRegex: .*@example\.com
{{< /highlight >}}</details> | |
|`rekorURL` |string |Rekor transparency log URL (optional, defaults to "https://rekor.sigstore.dev").  | |






### publicKey {#ImageVerificationConfig.rules..publicKey}

ImagePublicKeyVerifierV1Alpha1 configures a signature verification provider using a static public key.




| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`certificate` |string |A public certificate in PEM format accepted for image signature verification.  | |










