---
description: |
    KubeAuthenticationWebhookConfig configures kube-apiserver webhook token authentication.
    Bearer tokens which are not recognized by other authenticators are sent to the webhook
    (a `TokenReview` request) to be authenticated, e.g. for
    [aws-iam-authenticator](https://github.com/kubernetes-sigs/aws-iam-authenticator).

    Talos writes the kubeconfig to a file inside the kube-apiserver static pod and sets the
    `--authentication-token-webhook-*` flags of kube-apiserver accordingly,
    so these flags are not allowed in `KubeAPIServerConfig` `extraArgs`.

    See [webhook token authentication](https://kubernetes.io/docs/reference/access-authn-authz/authentication/#webhook-token-authentication).
title: KubeAuthenticationWebhookConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: KubeAuthenticationWebhookConfig
kubeconfig: | # Kubeconfig kube-apiserver uses to connect to the authentication webhook.
    apiVersion: v1
    kind: Config
    clusters:
      - name: aws-iam-authenticator
        cluster:
          server: https://127.0.0.1:21362/authenticate
          certificate-authority-data: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCi4uLgotLS0tLUVORCBDRVJUSUZJQ0FURS0tLS0tCg==
    users:
      - name: apiserver
    contexts:
      - name: webhook
        context:
          cluster: aws-iam-authenticator
          user: apiserver
    current-context: webhook
version: v1beta1 # The API version of the `authentication.k8s.io` `TokenReview` to send to and expect from the webhook.
cacheTTL: 5m0s # The duration to cache responses from the webhook token authenticator.
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`kubeconfig` |string |Kubeconfig kube-apiserver uses to connect to the authentication webhook.<br><br>The kubeconfig should be self-contained: kube-apiserver can't access any files on the host,<br>so certificates and keys should be specified inline (`certificate-authority-data`, `client-certificate-data`,<br>`client-key-data`), and `tokenFile`, `exec` and `auth-provider` are not supported.<br>If the certificate authority is not specified, kube-apiserver uses the trusted root CAs of the machine.  | |
|`version` |string |The API version of the `authentication.k8s.io` `TokenReview` to send to and expect from the webhook.<br><br>Defaults to the kube-apiserver default (`v1beta1`).  |`v1`<br />`v1beta1`<br /> |
|`cacheTTL` |Duration |The duration to cache responses from the webhook token authenticator.<br><br>Defaults to the kube-apiserver default (2 minutes), set to `0s` to disable caching.  | |






