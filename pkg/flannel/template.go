// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Code generated from the manifest https://raw.githubusercontent.com/flannel-io/flannel/v0.27.0/Documentation/kube-flannel.yml DO NOT EDIT

package flannel

// Template is a flannel manifest template.
var Template = []byte(`apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  labels:
    k8s-app: flannel
  name: flannel
rules:
- apiGroups:
  - ""
  resources:
  - pods
  verbs:
  - get
- apiGroups:
  - ""
  resources:
  - nodes
  verbs:
  - get
  - list
  - watch
- apiGroups:
  - ""
  resources:
  - nodes/status
  verbs:
  - patch
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  labels:
    k8s-app: flannel
  name: flannel
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: flannel
subjects:
- kind: ServiceAccount
  name: flannel
  namespace: kube-system
---
apiVersion: v1
kind: ServiceAccount
metadata:
  labels:
    k8s-app: flannel
  name: flannel
  namespace: kube-system
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: kube-flannel-cfg
  namespace: kube-system
  labels:
    tier: node
    k8s-app: flannel
data:
  cni-conf.json: |
    {
      "name": "cbr0",
      "cniVersion": "1.0.0",
      "plugins": [
        {
          "type": "flannel",
          "delegate": {
            "hairpinMode": true,
            "isDefaultGateway": true
          }
        },
        {
          "type": "portmap",
          "capabilities": {
            "portMappings": true
          }
        }
      ]
    }
  net-conf.json: |
    {
      {{- $hasIPv4 := false }}
      {{- range $cidr := .PodCIDRs }}
        {{- if contains $cidr "." }}
        {{- $hasIPv4 = true }}
      "Network": "{{ $cidr }}",
        {{- else }}
      "IPv6Network": "{{ $cidr }}",
      "EnableIPv6": true,
        {{- end }}
      {{- end }}
      {{- if not $hasIPv4 }}
      "EnableIPv4": false,
      {{- end }}
      "Backend": {
        "Type": "vxlan",
        "Port": 4789
      }
    }
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  labels:
    k8s-app: flannel
    tier: node
  name: kube-flannel
  namespace: kube-system
spec:
  selector:
    matchLabels:
      k8s-app: flannel
      tier: node
  template:
    metadata:
      labels:
        k8s-app: flannel
        tier: node
    spec:
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: kubernetes.io/os
                operator: In
                values:
                - linux
      containers:
      - args:
        - --ip-masq
        - --kube-subnet-mgr
        {{- range $arg := .FlannelExtraArgs }}
        - {{ $arg | json }}
        {{- end }}
        command:
        - /opt/bin/flanneld
        env:
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        - name: EVENT_QUEUE_DEPTH
          value: "5000"
        {{- if .FlannelKubeServiceHost }}
        - name: KUBERNETES_SERVICE_HOST
          value: {{ .FlannelKubeServiceHost | json }}
        {{- end }}
        {{- if .FlannelKubeServicePort }}
        - name: KUBERNETES_SERVICE_PORT
          value: {{ .FlannelKubeServicePort | json }}
        {{- end }}
        image: '{{ .FlannelImage }}'
        name: kube-flannel
        resources:
          requests:
            cpu: 100m
            memory: 50Mi
        securityContext:
          capabilities:
            add:
            - NET_ADMIN
            - NET_RAW
          privileged: false
        volumeMounts:
        - mountPath: /run/flannel
          name: run
        - mountPath: /etc/kube-flannel/
          name: flannel-cfg
      hostNetwork: true
      initContainers:
      - args:
        - -f
        - /etc/kube-flannel/cni-conf.json
        - /etc/cni/net.d/10-flannel.conflist
        command:
        - cp
        image: '{{ .FlannelImage }}'
        name: install-config
        resources: {}
        volumeMounts:
        - mountPath: /etc/cni/net.d
          name: cni
        - mountPath: /etc/kube-flannel/
          name: flannel-cfg
      priorityClassName: system-node-critical
      serviceAccountName: flannel
      tolerations:
      - effect: NoSchedule
        operator: Exists
      - effect: NoExecute
        operator: Exists
      volumes:
      - hostPath:
          path: /run/flannel
        name: run
      - hostPath:
          path: /opt/cni/bin
        name: cni-plugin
      - hostPath:
          path: /etc/cni/net.d
        name: cni
      - configMap:
          name: kube-flannel-cfg
        name: flannel-cfg
  updateStrategy: {}
---
`)
