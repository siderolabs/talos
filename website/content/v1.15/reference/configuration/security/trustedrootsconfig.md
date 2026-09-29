---
description: TrustedRootsConfig allows to configure additional trusted CA roots.
title: TrustedRootsConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: TrustedRootsConfig
name: my-enterprise-ca # Name of the config document.
certificates: | # List of additional trusted certificate authorities (as PEM-encoded certificates).
    -----BEGIN CERTIFICATE-----
    ...
    -----END CERTIFICATE-----
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of the config document.  | |
|`certificates` |string |List of additional trusted certificate authorities (as PEM-encoded certificates).<br><br>Multiple certificates can be provided in a single config document, separated by newline characters.  | |






