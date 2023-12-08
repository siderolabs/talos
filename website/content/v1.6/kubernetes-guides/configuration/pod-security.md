---
title: "Pod Security"
description: "Enabling Pod Security Admission plugin to configure Pod Security Standards."
aliases:
  - ../../guides/pod-security
---

Kubernetes deprecated [Pod Security Policy](https://kubernetes.io/docs/concepts/policy/pod-security-policy/) as of v1.21, and it was removed in v1.25.

Pod Security Policy was replaced with [Pod Security Admission](https://kubernetes.io/docs/concepts/security/pod-security-admission/), which is enabled by default
starting with Kubernetes v1.23.

Talos Linux by default enables and configures Pod Security Admission plugin to enforce [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/) with the
`baseline` profile as the default enforced with the exception of `kube-system` namespace which enforces `privileged` profile.

Some applications (e.g. Prometheus node exporter or storage solutions) require more relaxed Pod Security Standards, which can be configured by either updating the Pod Security Admission plugin configuration,
or by using the `pod-security.kubernetes.io/enforce` label on the namespace level:

```shell
kubectl label namespace NAMESPACE-NAME pod-security.kubernetes.io/enforce=privileged
```

## Configuration

Talos provides default Pod Security Admission in the machine configuration:

```yaml
apiVersion: pod-security.admission.config.k8s.io/v1alpha1
kind: PodSecurityConfiguration
defaults:
    enforce: "baseline"
    enforce-version: "latest"
    audit: "restricted"
    audit-version: "latest"
    warn: "restricted"
    warn-version: "latest"
exemptions:
    usernames: []
    runtimeClasses: []
    namespaces: [kube-system]
```

This is a cluster-wide configuration for the Pod Security Admission plugin:

* by default `baseline` [Pod Security Standard](https://kubernetes.io/docs/concepts/security/pod-security-standards/) profile is enforced
* more strict `restricted` profile is not enforced, but API server warns about found issues

This default policy can be modified by updating the generated machine configuration before the cluster is created or on the fly by using the `talosctl` CLI utility.

Verify current admission plugin configuration with:

```shell
$ talosctl get admissioncontrolconfigs.kubernetes.talos.dev admission-control -o yaml
node: 172.20.0.2
metadata:
    namespace: controlplane
    type: AdmissionControlConfigs.kubernetes.talos.dev
    id: admission-control
    version: 1
    owner: config.K8sControlPlaneController
    phase: running
    created: 2022-02-22T20:28:21Z
    updated: 2022-02-22T20:28:21Z
spec:
    config:
        - name: PodSecurity
          configuration:
            apiVersion: pod-security.admission.config.k8s.io/v1alpha1
            defaults:
                audit: restricted
                audit-version: latest
                enforce: baseline
                enforce-version: latest
                warn: restricted
                warn-version: latest
            exemptions:
                namespaces:
                    - kube-system
                runtimeClasses: []
                usernames: []
            kind: PodSecurityConfiguration
```

## Usage

Create a deployment that satisfies the `baseline` policy but gives warnings on `restricted` policy:

```shell
$ kubectl create deployment nginx --image=nginx
Warning: would violate PodSecurity "restricted:latest": allowPrivilegeEscalation != false (container "nginx" must set securityContext.allowPrivilegeEscalation=false), unrestricted capabilities (container "nginx" must set securityContext.capabilities.drop=["ALL"]), runAsNonRoot != true (pod or container "nginx" must set securityContext.runAsNonRoot=true), seccompProfile (pod or container "nginx" must set securityContext.seccompProfile.type to "RuntimeDefault" or "Localhost")
deployment.apps/nginx created
$ kubectl get pods
NAME                     READY   STATUS    RESTARTS   AGE
nginx-85b98978db-j68l8   1/1     Running   0          2m3s
```

Create a daemonset which fails to meet requirements of the `baseline` policy:

```yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  labels:
    app: debug-container
  name: debug-container
  namespace: default
spec:
  revisionHistoryLimit: 10
  selector:
    matchLabels:
      app: debug-container
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: debug-container
    spec:
      containers:
      - args:
        - "360000"
        command:
        - /bin/sleep
        image: ubuntu:latest
        imagePullPolicy: IfNotPresent
        name: debug-container
        resources: {}
        securityContext:
          privileged: true
        terminationMessagePath: /dev/termination-log
        terminationMessagePolicy: File
      dnsPolicy: ClusterFirstWithHostNet
      hostIPC: true
      hostPID: true
      hostNetwork: true
      restartPolicy: Always
      schedulerName: default-scheduler
      securityContext: {}
      terminationGracePeriodSeconds: 30
  updateStrategy:
    rollingUpdate:
      maxSurge: 0
      maxUnavailable: 1
    type: RollingUpdate
```

```shell
$ kubectl apply -f debug.yaml
Warning: would violate PodSecurity "restricted:latest": host namespaces (hostNetwork=true, hostPID=true, hostIPC=true), privileged (container "debug-container" must not set securityContext.privileged=true), allowPrivilegeEscalation != false (container "debug-container" must set securityContext.allowPrivilegeEscalation=false), unrestricted capabilities (container "debug-container" must set securityContext.capabilities.drop=["ALL"]), runAsNonRoot != true (pod or container "debug-container" must set securityContext.runAsNonRoot=true), seccompProfile (pod or container "debug-container" must set securityContext.seccompProfile.type to "RuntimeDefault" or "Localhost")
daemonset.apps/debug-container created
```

Daemonset `debug-container` gets created, but no pods are scheduled:

```shell
$ kubectl get ds
NAME              DESIRED   CURRENT   READY   UP-TO-DATE   AVAILABLE   NODE SELECTOR   AGE
debug-container   0         0         0       0            0           <none>          34s
```

Pod Security Admission plugin errors are in the daemonset events:

```shell
$ kubectl describe ds debug-container
...
  Warning  FailedCreate  92s                daemonset-controller  Error creating: pods "debug-container-kwzdj" is forbidden: violates PodSecurity "baseline:latest": host namespaces (hostNetwork=true, hostPID=true, hostIPC=true), privileged (container "debug-container" must not set securityContext.privileged=true)
```

Pod Security Admission configuration can also be overridden on a namespace level:

```shell
$ kubectl label ns default pod-security.kubernetes.io/enforce=privileged
namespace/default labeled
$ kubectl get ds
NAME              DESIRED   CURRENT   READY   UP-TO-DATE   AVAILABLE   NODE SELECTOR   AGE
debug-container   2         2         0       2            0           <none>          4s
```

As enforce policy was updated to the `privileged` for the `default` namespace, `debug-container` is now successfully running.
