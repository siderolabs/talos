---
description: KubeStaticPodConfig configures a pod definition to be run as a static
    pod by the kubelet.
title: KubeStaticPodConfig
---

<!-- markdownlint-disable -->









{{< highlight yaml >}}
apiVersion: v1alpha1
kind: KubeStaticPodConfig
name: nginx # Name of the static pod.
# Static pods can be used to run components which should be started before the Kubernetes control plane is up.
pod:
    apiVersion: v1
    kind: Pod
    metadata:
        name: nginx
    spec:
        containers:
            - image: nginx
              name: nginx
{{< /highlight >}}


| Field | Type | Description | Value(s) |
|-------|------|-------------|----------|
|`name` |string |Name of the static pod.  | |
|`pod` |Unstructured |Static pods can be used to run components which should be started before the Kubernetes control plane is up.<br>Talos doesn't validate the pod definition.<br>Updates to this field can be applied without a reboot.<br><br>See https://kubernetes.io/docs/tasks/configure-pod-container/static-pod/.  | |






