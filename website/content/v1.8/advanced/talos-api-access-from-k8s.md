---
title: "Talos API access from Kubernetes"
description: "How to access Talos API from within Kubernetes."
aliases:
  - ../guides/talos-api-access-from-k8s
---

In this guide, we will enable the Talos feature to access the Talos API from within Kubernetes.

## Enabling the Feature

Edit the machine configuration to enable the feature, specifying the Kubernetes namespaces from which Talos API
can be accessed and the allowed Talos API roles.

```bash
talosctl -n 172.20.0.2 edit machineconfig
```

Configure the `kubernetesTalosAPIAccess` like the following:

```yaml
spec:
  machine:
    features:
      kubernetesTalosAPIAccess:
        enabled: true
        allowedRoles:
          - os:reader
        allowedKubernetesNamespaces:
          - default
```

## Injecting Talos ServiceAccount into manifests

Create the following manifest file `deployment.yaml`:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: talos-api-access
spec:
  selector:
    matchLabels:
      app: talos-api-access
  template:
    metadata:
      labels:
        app: talos-api-access
    spec:
      containers:
        - name: talos-api-access
          image: alpine:3
          command:
            - sh
            - -c
            - |
              wget -O /usr/local/bin/talosctl https://github.com/siderolabs/talos/releases/download/{{< release >}}/talosctl-linux-amd64
              chmod +x /usr/local/bin/talosctl
              while true; talosctl -n 172.20.0.2 version; do sleep 1; done
```

**Note:** make sure that you replace the IP `172.20.0.2` with a valid Talos node IP.

Use `talosctl inject serviceaccount` command to inject the Talos ServiceAccount into the manifest.

```bash
talosctl inject serviceaccount -f deployment.yaml > deployment-injected.yaml
```

Inspect the generated manifest:

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  creationTimestamp: null
  name: talos-api-access
spec:
  selector:
    matchLabels:
      app: talos-api-access
  strategy: {}
  template:
    metadata:
      creationTimestamp: null
      labels:
        app: talos-api-access
    spec:
      containers:
      - command:
        - sh
        - -c
        - |
          wget -O /usr/local/bin/talosctl https://github.com/siderolabs/talos/releases/download/{{< release >}}/talosctl-linux-amd64
          chmod +x /usr/local/bin/talosctl
          while true; talosctl -n 172.20.0.2 version; do sleep 1; done
        image: alpine:3
        name: talos-api-access
        resources: {}
        volumeMounts:
        - mountPath: /var/run/secrets/talos.dev
          name: talos-secrets
      tolerations:
      - operator: Exists
      volumes:
      - name: talos-secrets
        secret:
          secretName: talos-api-access-talos-secrets
status: {}
---
apiVersion: talos.dev/v1alpha1
kind: ServiceAccount
metadata:
    name: talos-api-access-talos-secrets
spec:
    roles:
        - os:reader
---
```

As you can notice, your deployment manifest is now injected with the Talos ServiceAccount.

## Testing API Access

Apply the new manifest into `default` namespace:

```bash
kubectl apply -n default -f deployment-injected.yaml
```

Follow the logs of the pods belong to the deployment:

```bash
kubectl logs -n default -f -l app=talos-api-access
```

You'll see a repeating output similar to the following:

```text
Client:
    Tag:         <talos version>
    SHA:         ....
    Built:
    Go version:  go1.18.4
    OS/Arch:     linux/amd64
Server:
    NODE:        172.20.0.2
    Tag:         <talos version>
    SHA:         ...
    Built:
    Go version:  go1.18.4
    OS/Arch:     linux/amd64
    Enabled:     RBAC
```

This means that the pod can talk to Talos API of node 172.20.0.2 successfully.
