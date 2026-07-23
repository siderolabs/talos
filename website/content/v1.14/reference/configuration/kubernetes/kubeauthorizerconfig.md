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
    connectionInfo:
        type: InClusterConfig
    failurePolicy: Deny
    matchConditionSubjectAccessReviewVersion: v1
    matchConditions:
        - expression: has(request.resourceAttributes)
        - expression: '!(\''system:serviceaccounts:kube-system\'' in request.groups)'
    subjectAccessReviewVersion: v1
    timeout: 3s
{{< /highlight >}}

{{< highlight yaml >}}
apiVersion: v1alpha1
kind: KubeAuthorizerConfig
name: in-cluster-authorizer # Name of the authorizer, should be be DNS1123 labels like myauthorizername or subdomains like myauthorizer.example.domain.
type: Webhook # Type is the name of the authorizer.
# Webhook is the configuration for the webhook authorizer.
webhook:
    connectionInfo:
        type: InClusterConfig
    failurePolicy: NoOpinion
    matchConditionSubjectAccessReviewVersion: v1
    subjectAccessReviewVersion: v1
    timeout: 3s
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of the authorizer, should be be DNS1123 labels like myauthorizername or subdomains like myauthorizer.example.domain.  | |
|`type` |string |Type is the name of the authorizer.  |`Node`<br />`RBAC`<br />`Webhook`<br /> |
|`webhook` |Unstructured |Webhook is the configuration for the webhook authorizer.<br><br>This field is required if the AuthorizerType is Webhook, should not be set for other authorizer types.<br>The value is the literal Kubernetes webhook authorizer configuration.  | |






