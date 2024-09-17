---
title: "Device Plugins"
description: "In this guide you will learn how to expose host devices to the Kubernetes pods."
---

[Kubernetes Device Plugins](https://kubernetes.io/docs/concepts/extend-kubernetes/compute-storage-net/device-plugins/) can be used to expose host devices to the Kubernetes pods.
This guide will show you how to deploy a device plugin to your Talos cluster.
In this guide, we will use [Kubernetes Generic Device Plugin](https://github.com/squat/generic-device-plugin), but there are other implementations available.

## Deploying the Device Plugin

The Kubernetes Generic Device Plugin is a DaemonSet that runs on each node in the cluster, exposing the devices to the pods.
The device plugin is configured with a [list of devices to expose](https://github.com/squat/generic-device-plugin#overview), e.g.
`--device='{"name": "video", "groups": [{"paths": [{"path": "/dev/video0"}]}]}`.

In this guide, we will demonstrate how to deploy the device plugin with a configuration that exposes the `/dev/net/tun` device.
This device is commonly used for user-space Wireguard, including Tailscale.

```yaml
# generic-device-plugin.yaml
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: generic-device-plugin
  namespace: kube-system
  labels:
    app.kubernetes.io/name: generic-device-plugin
spec:
  selector:
    matchLabels:
      app.kubernetes.io/name: generic-device-plugin
  template:
    metadata:
      labels:
        app.kubernetes.io/name: generic-device-plugin
    spec:
      priorityClassName: system-node-critical
      tolerations:
      - operator: "Exists"
        effect: "NoExecute"
      - operator: "Exists"
        effect: "NoSchedule"
      containers:
      - image: squat/generic-device-plugin
        args:
        - --device
        - |
          name: tun
          groups:
            - count: 1000
              paths:
                - path: /dev/net/tun
        name: generic-device-plugin
        resources:
          requests:
            cpu: 50m
            memory: 10Mi
          limits:
            cpu: 50m
            memory: 20Mi
        ports:
        - containerPort: 8080
          name: http
        securityContext:
          privileged: true
        volumeMounts:
        - name: device-plugin
          mountPath: /var/lib/kubelet/device-plugins
        - name: dev
          mountPath: /dev
      volumes:
      - name: device-plugin
        hostPath:
          path: /var/lib/kubelet/device-plugins
      - name: dev
        hostPath:
          path: /dev
  updateStrategy:
    type: RollingUpdate
```

Apply the manifest to your cluster:

```sh
kubectl apply -f generic-device-plugin.yaml
```

Once the device plugin is deployed, you can verify that the nodes have a new resource: `squat.ai/tun` (the `tun` name comes from the name of the group in the device plugin configuration).:

```sh
$ kubectl describe node worker-1
...
Allocated resources:
  Resource           Requests     Limits
  --------           --------     ------
  ...
  squat.ai/tun       0            0
```

## Deploying a Pod with the Device

Now that the device plugin is deployed, you can deploy a pod that requests the device.
The request for the device is specified as a [resource](https://kubernetes.io/docs/concepts/configuration/manage-resources-containers/) in the pod spec.

```yaml
requests:
  limits:
    squat.ai/tun: "1"
```

Here is an example non-privileged pod spec that requests the `/dev/net/tun` device:

```yaml
# tun-pod.yaml
apiVersion: v1
kind: Pod
metadata:
  name: tun-test
spec:
  containers:
  - image: alpine
    name: test
    command:
      - sleep
      - inf
    resources:
      limits:
        squat.ai/tun: "1"
    securityContext:
      allowPrivilegeEscalation: false
      capabilities:
        drop:
          - ALL
        add:
          - NET_ADMIN
  dnsPolicy: ClusterFirst
  restartPolicy: Always
```

When running the pod, you should see the `/dev/net/tun` device available:

```sh
$ ls -l /dev/net/tun
crw-rw-rw-    1 root     root       10, 200 Sep 17 10:30 /dev/net/tun
```
