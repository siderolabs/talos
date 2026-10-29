---
description: KubeAuthorizerConfig configures kube-apiserver authorization by configuring
    a specific authorization plugin.
title: KubeAuthorizerConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: KubeAuthorizerConfig
name: node # Name of the authorizer, should be be DNS1123 labels like myauthorizername or subdomains like myauthorizer.example.domain.
type: Node # Type is the name of the authorizer.
{{< /highlight >}}

{{< highlight yaml >}}
apiVersion: v1alpha1
kind: KubeAuthorizerConfig
name: rbac # Name of the authorizer, should be be DNS1123 labels like myauthorizername or subdomains like myauthorizer.example.domain.
type: RBAC # Type is the name of the authorizer.
{{< /highlight >}}

{{< highlight yaml >}}
apiVersion: v1alpha1
kind: KubeAuthorizerConfig
name: webhook # Name of the authorizer, should be be DNS1123 labels like myauthorizername or subdomains like myauthorizer.example.domain.
type: Webhook # Type is the name of the authorizer.
# Webhook is the configuration for the webhook authorizer.
webhook:
    failurePolicy: Deny
    matchConditionSubjectAccessReviewVersion: v1
    matchConditions:
        - expression: has(request.resourceAttributes)
        - expression: '!(\''system:serviceaccounts:kube-system\'' in request.groups)'
    subjectAccessReviewVersion: v1
    timeout: 3s
kubeconfig: | # Kubeconfig kube-apiserver uses to connect to the webhook authorizer.
    apiVersion: v1
    kind: Config
    clusters:
      - name: webhook
        cluster:
          server: https://authz.example.com/authorize
          certificate-authority-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCi4uLgotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==
    users:
      - name: kube-apiserver
        user:
          token: webhook-token
    contexts:
      - name: webhook
        context:
          cluster: webhook
          user: kube-apiserver
    current-context: webhook
{{< /highlight >}}

{{< highlight yaml >}}
apiVersion: v1alpha1
kind: KubeAuthorizerConfig
name: local-authorizer # Name of the authorizer, should be be DNS1123 labels like myauthorizername or subdomains like myauthorizer.example.domain.
type: Webhook # Type is the name of the authorizer.
# Webhook is the configuration for the webhook authorizer.
webhook:
    failurePolicy: NoOpinion
    matchConditionSubjectAccessReviewVersion: v1
    subjectAccessReviewVersion: v1
    timeout: 3s
kubeconfig: | # Kubeconfig kube-apiserver uses to connect to the webhook authorizer.
    apiVersion: v1
    kind: Config
    clusters:
      - name: local-authorizer
        cluster:
          server: http://127.0.0.1:9051/authorize
    users:
      - name: local-authorizer
    contexts:
      - name: local-authorizer
        context:
          cluster: local-authorizer
          user: local-authorizer
    current-context: local-authorizer
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of the authorizer, should be be DNS1123 labels like myauthorizername or subdomains like myauthorizer.example.domain.  | |
|`type` |string |Type is the name of the authorizer.  |`Node`<br />`RBAC`<br />`Webhook`<br /> |
|`webhook` |Unstructured |Webhook is the configuration for the webhook authorizer.<br><br>This field is required if the AuthorizerType is Webhook, should not be set for other authorizer types.<br>The value is the literal Kubernetes webhook authorizer configuration.<br><br>When `kubeconfig` is set, `connectionInfo` should be omitted, as Talos fills it in.  | |
|`kubeconfig` |string |Kubeconfig kube-apiserver uses to connect to the webhook authorizer.<br><br>Talos writes the kubeconfig to a file inside the kube-apiserver static pod and sets<br>`webhook.connectionInfo` to `type: KubeConfigFile` pointing to that file.<br>This field can only be set for the Webhook authorizer type.<br><br>The kubeconfig should be self-contained: kube-apiserver can't access any files on the host,<br>so certificates and keys should be specified inline (`certificate-authority-data`, `client-certificate-data`,<br>`client-key-data`), and `tokenFile`, `exec` and `auth-provider` are not supported.<br>If the certificate authority is not specified, kube-apiserver uses the trusted root CAs of the machine.  | |






