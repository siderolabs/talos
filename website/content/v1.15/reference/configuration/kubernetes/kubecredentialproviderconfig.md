---
description: KubeCredentialProviderConfig configures kubelet's credential provider.
title: KubeCredentialProviderConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: KubeCredentialProviderConfig
# Kubelet credential provider configuration (used for image registry authentication).
configuration:
    apiVersion: kubelet.config.k8s.io/v1
    kind: CredentialProviderConfig
    providers:
        - apiVersion: credentialprovider.kubelet.k8s.io/v1
          defaultCacheDuration: 12h
          matchImages:
            - '*.dkr.ecr.*.amazonaws.com'
            - '*.dkr.ecr.*.amazonaws.com.cn'
            - '*.dkr.ecr-fips.*.amazonaws.com'
            - '*.dkr.ecr.us-iso-east-1.c2s.ic.gov'
            - '*.dkr.ecr.us-isob-east-1.sc2s.sgov.gov'
          name: ecr-credential-provider

# # Additional configuration files for the credential provider plugins.
# configFiles:
#     - name: azure.json # Name of the file in the `/etc/kubernetes/credentialproviders` directory.
#       content: | # Content of the file.
#         {"cloud": "AzurePublicCloud", "useManagedIdentityExtension": true}
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`configuration` |Unstructured |Kubelet credential provider configuration (used for image registry authentication).<br>The value is the literal kubelet's credential provider configuration.  | |
|`configFiles` |<a href="#KubeCredentialProviderConfig.configFiles.">[]KubeCredentialProviderConfigFileV1Alpha1</a> |Additional configuration files for the credential provider plugins.<br><br>Each file is written to the `/etc/kubernetes/credentialproviders` directory<br>with the permissions `0600`, so it can be referenced in the `args` of a credential provider.<br><br>Requires `configuration` to be set. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
configFiles:
    - name: azure.json # Name of the file in the `/etc/kubernetes/credentialproviders` directory.
      content: | # Content of the file.
        {"cloud": "AzurePublicCloud", "useManagedIdentityExtension": true}
{{< /highlight >}}</details> | |




## configFiles[] {#KubeCredentialProviderConfig.configFiles.}

KubeCredentialProviderConfigFileV1Alpha1 is a configuration file for a credential provider plugin.



{{< highlight yaml >}}
configFiles:
    - name: azure.json # Name of the file in the `/etc/kubernetes/credentialproviders` directory.
      content: | # Content of the file.
        {"cloud": "AzurePublicCloud", "useManagedIdentityExtension": true}
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of the file in the `/etc/kubernetes/credentialproviders` directory. <details><summary>Show example(s)</summary>{{< highlight yaml >}}
name: azure.json
{{< /highlight >}}</details> | |
|`content` |string |Content of the file.  | |








