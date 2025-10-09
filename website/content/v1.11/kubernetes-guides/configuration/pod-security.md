---
title: "Pod Security"
description: "Enabling Pod Security Admission plugin to configure Pod Security Standards."
aliases:
  - ../../guides/pod-security
---

Kubernetes deprecated [Pod Security Policy (PSP)](https://kubernetes.io/docs/concepts/policy/pod-security-policy/) in version 1.21 and removed it entirely in 1.25.

It was replaced by [Pod Security Admission (PSA)](https://kubernetes.io/docs/concepts/security/pod-security-admission/), which is enabled by default starting with v1.23.

Talos Linux automatically enables and configures PSA to enforce Pod Security Standards.
These [Pod Security Standards](https://kubernetes.io/docs/concepts/security/pod-security-standards/) define three policies that cover the security spectrum:

* **Privileged**: Unrestricted policy, providing the widest possible level of permissions.
* **Baseline**: Minimally restrictive policy.
* **Restricted**: Heavily restricted policy.

By default, Talos with the help of PSA, applies the `baseline` profile to all namespaces, except for the `kube-system` namespace, which uses the `privileged` profile.

## Default PSA Configuration

Here is the default PSA configuration on Talos:

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

This cluster-wide configuration:

* Enforces the `baseline` security profile by default.
* Throws a warning, if the `restricted` profile is violated, but does not enforce this profile.

## Modify the Default PSA Configuraion

You can modify this PSA policy by updating the generated machine configuration before the cluster is created or on the fly by using the `talosctl` CLI utility.

Verify current admission plugin configuration with:

```bash
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

## Workloads That Satisfy the Different Security Profiles

To deploy a workload that satisfies both the `baseline` and `restricted` profiles, you must ensure that your workloads:

* Run as non-root users (UID 1000 or higher)
* Use read-only root filesystems where possible
* Minimize or eliminate kernel capabilities

To see how PSA treats workloads that violate security profiles, consider these examples that violate the `restricted`, `baseline`, or both profiles:

* A Deployment that satisfies the `restricted` profile
* A Deployment that meets `baseline` requirements but `violates` restricted
* A DaemonSet that violates both `restricted` and `baseline` profiles

### Deployment that Satisfies the Restricted Profile

This Deployment complies with the `restricted` profile and does not produce any errors or warnings when applied:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: example-workload
  namespace: default
spec:
  selector:
    matchLabels:
      app: example-workload
  template:
    metadata:
      labels:
        app: example-workload
    spec:
      securityContext:
        runAsNonRoot: true
        runAsUser: 1000
        runAsGroup: 1000
        fsGroup: 1000
        seccompProfile:
          type: RuntimeDefault
      containers:
        - name: example-workload
          image: ghcr.io/siderolabs/example-workload
          imagePullPolicy: Always
          resources:
            requests:
              cpu: 100m
              memory: 128Mi
            limits:
              cpu: 500m
              memory: 256Mi
          securityContext:
            allowPrivilegeEscalation: false
            runAsNonRoot: true
            capabilities:
              drop:
                - ALL
```

When you apply this `example-workload` Deployment, it successfully creates the Deployment and deploys its pods:

<pre>
$ kubectl apply -f example-workload.yaml
deployment.apps/example-workload created

$ kubectl get pods
NAME                                READY   STATUS    RESTARTS   AGE
example-workload-6f847d64b9-jctkv   1/1     Running   0          10s

</pre>

This is because the Deployment follows Talos’ recommended security practices, which, as shown in the Deployment configuration, include:

* **runAsNonRoot: true**: Prevents the container from running as root.
* **runAsUser and runAsGroup**: Ensures a dedicated non-root user (UID/GID 1000) runs the process.
* **fsGroup**: Sets file system group ownership for shared volumes.
* **seccompProfile: RuntimeDefault**: Uses the default seccomp profile to restrict available system calls.
* **allowPrivilegeEscalation: false**: Blocks processes from gaining additional privileges.
* **capabilities: drop: [ALL]**: Removes unnecessary Linux capabilities.

### Deployment that Violates the Restricted but Meets Baseline Profile

Run the following command to create a Deployment that complies with the `baseline` profile but violates the `restricted` profile:

```bash
kubectl create deployment nginx --image=nginx
```

Applying this Deployment triggers warnings indicating excessive privileges:

<pre>
Warning: would violate PodSecurity "restricted:latest": allowPrivilegeEscalation != false (container "nginx" must set securityContext.allowPrivilegeEscalation=false), unrestricted capabilities (container "nginx" must set securityContext.capabilities.drop=["ALL"]), runAsNonRoot != true (pod or container "nginx" must set securityContext.runAsNonRoot=true), seccompProfile (pod or container "nginx" must set securityContext.seccompProfile.type to "RuntimeDefault" or "Localhost")
deployment.apps/nginx created
</pre>

Despite these warnings, the deployment and its pods are still created successfully because it complies with the default Talos `baseline` security profile:

<pre>
$ kubectl get pods
NAME                     READY   STATUS    RESTARTS   AGE
nginx-85b98978db-j68l8   1/1     Running   0          2m3s
</pre>

### DaemonSet that Fails Both the Restricted and Baseline Profiles

This DaemonSet violates both the `baseline` and `restricted` profiles:

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

When you apply this DaemonSet:

* An error is thrown, showing that the DaemonSet requests too much privileges:

<pre>
Warning: would violate PodSecurity "restricted:latest": host namespaces (hostNetwork=true, hostPID=true, hostIPC=true), privileged (container "debug-container" must not set securityContext.privileged=true), allowPrivilegeEscalation != false (container "debug-container" must set securityContext.allowPrivilegeEscalation=false), unrestricted capabilities (container "debug-container" must set securityContext.capabilities.drop=["ALL"]), runAsNonRoot != true (pod or container "debug-container" must set securityContext.runAsNonRoot=true), seccompProfile (pod or container "debug-container" must set securityContext.seccompProfile.type to "RuntimeDefault" or "Localhost")
daemonset.apps/debug-container created
</pre>

* The DaemonSet object gets created but no pods are scheduled:

<pre>
$ kubectl get ds

NAME              DESIRED   CURRENT   READY   UP-TO-DATE   AVAILABLE   NODE SELECTOR   AGE
debug-container   0         0         0       0            0           <none>          34s

</pre>

* When you describe the DaemonSet, the `Events` section shows that Pod Security Admission errors are blocking pod creation:

<pre>
$ kubectl describe ds debug-container
...
Warning  FailedCreate  92s                daemonset-controller  Error creating: pods "debug-container-kwzdj" is forbidden: violates PodSecurity "baseline:latest": host namespaces (hostNetwork=true, hostPID=true, hostIPC=true), privileged (container "debug-container" must not set securityContext.privileged=true)
</pre>

This happens because the DaemonSet does not comply with the enforced `baseline` Pod Security profile.

## Override the Pod Security Admission Configuration

You can override the Pod Security Admission configuration at the namespace level.

This is especially useful for applications like Prometheus node exporter or storage solutions that require more relaxed Pod Security Standards.

Using the DaemonSet workload example, you can update the enforced policy to `privileged` for its namespace, which is the default namespace.

```bash
kubectl label ns default pod-security.kubernetes.io/enforce=privileged
namespace/default labeled
```

With this update, the DaemonSet is successfully running:

<pre>
$ kubectl get ds

NAME              DESIRED   CURRENT   READY   UP-TO-DATE   AVAILABLE   NODE SELECTOR   AGE
debug-container   2         2         0       2            0           <none>          4s
</pre>
