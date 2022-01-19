// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

// kube-apiserver configuration:

var kubeSystemEncryptionConfigTemplate = []byte(`apiVersion: v1
kind: EncryptionConfig
resources:
- resources:
  - secrets
  providers:
  - aescbc:
      keys:
      - name: key1
        secret: {{ .Root.AESCBCEncryptionSecret }}
  - identity: {}
`)

var kubeSystemAuditPolicyTemplate = []byte(`apiVersion: audit.k8s.io/v1beta1
kind: Policy
rules:
- level: Metadata
`)

// manifests injected into kube-apiserver

var kubeletBootstrappingToken = []byte(`apiVersion: v1
kind: Secret
metadata:
  name: bootstrap-token-{{ .Secrets.BootstrapTokenID }}
  namespace: kube-system
type: bootstrap.kubernetes.io/token
stringData:
  token-id: "{{ .Secrets.BootstrapTokenID }}"
  token-secret: "{{ .Secrets.BootstrapTokenSecret }}"
  usage-bootstrap-authentication: "true"

  # Extra groups to authenticate the token as. Must start with "system:bootstrappers:"
  auth-extra-groups: system:bootstrappers:nodes
`)

// csrNodeBootstrapTemplate lets bootstrapping tokens and nodes request CSRs.
var csrNodeBootstrapTemplate = []byte(`apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: system-bootstrap-node-bootstrapper
subjects:
- kind: Group
  name: system:bootstrappers:nodes
  apiGroup: rbac.authorization.k8s.io
- kind: Group
  name: system:nodes
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: system:node-bootstrapper
  apiGroup: rbac.authorization.k8s.io
`)

// csrApproverRoleBindingTemplate instructs the csrapprover controller to
// automatically approve CSRs made by bootstrapping tokens for client
// credentials.
//
// This binding should be removed to disable CSR auto-approval.
var csrApproverRoleBindingTemplate = []byte(`apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: system-bootstrap-approve-node-client-csr
subjects:
- kind: Group
  name: system:bootstrappers:nodes
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: system:certificates.k8s.io:certificatesigningrequests:nodeclient
  apiGroup: rbac.authorization.k8s.io
`)

// csrRenewalRoleBindingTemplate instructs the csrapprover controller to
// automatically approve all CSRs made by nodes to renew their client
// certificates.
//
// This binding should be altered in the future to hold a list of node
// names instead of targeting `system:nodes` so we can revoke individual
// node's ability to renew its certs.
var csrRenewalRoleBindingTemplate = []byte(`apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: system-bootstrap-node-renewal
subjects:
- kind: Group
  name: system:nodes
  apiGroup: rbac.authorization.k8s.io
roleRef:
  kind: ClusterRole
  name: system:certificates.k8s.io:certificatesigningrequests:selfnodeclient
  apiGroup: rbac.authorization.k8s.io
`)

var kubeSystemSARoleBindingTemplate = []byte(`apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: system:default-sa
subjects:
  - kind: ServiceAccount
    name: default
    namespace: kube-system
roleRef:
  kind: ClusterRole
  name: cluster-admin
  apiGroup: rbac.authorization.k8s.io
`)

var kubeProxyTemplate = []byte(`apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: kube-proxy
  namespace: kube-system
  labels:
    tier: node
    k8s-app: kube-proxy
spec:
  selector:
    matchLabels:
      tier: node
      k8s-app: kube-proxy
  template:
    metadata:
      labels:
        tier: node
        k8s-app: kube-proxy
    spec:
      containers:
      - name: kube-proxy
        image: {{ .ProxyImage }}
        command:
        - /usr/local/bin/kube-proxy
        {{- range $arg := .ProxyArgs }}
        - {{ $arg | json }}
        {{- end }}
        env:
          - name: NODE_NAME
            valueFrom:
              fieldRef:
                fieldPath: spec.nodeName
        securityContext:
          privileged: true
        volumeMounts:
        - mountPath: /lib/modules
          name: lib-modules
          readOnly: true
        - mountPath: /etc/ssl/certs
          name: ssl-certs-host
          readOnly: true
        - name: kubeconfig
          mountPath: /etc/kubernetes
          readOnly: true
      hostNetwork: true
      priorityClassName: system-cluster-critical
      serviceAccountName: kube-proxy
      tolerations:
      - effect: NoSchedule
        operator: Exists
      - effect: NoExecute
        operator: Exists
      volumes:
      - name: lib-modules
        hostPath:
          path: /lib/modules
      - name: ssl-certs-host
        hostPath:
          path: /etc/ssl/certs
      - name: kubeconfig
        configMap:
          name: kubeconfig-in-cluster
  updateStrategy:
    rollingUpdate:
      maxUnavailable: 1
    type: RollingUpdate
---
apiVersion: v1
kind: ServiceAccount
metadata:
  namespace: kube-system
  name: kube-proxy
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: kube-proxy
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:node-proxier # Automatically created system role.
subjects:
- kind: ServiceAccount
  name: kube-proxy
  namespace: kube-system
`)

// kubeConfigInCluster instructs clients to use their service account token,
// but unlike an in-cluster client doesn't rely on the `KUBERNETES_SERVICE_PORT`
// and `KUBERNETES_PORT` to determine the API servers address.
//
// This kubeconfig is used by bootstrapping pods that might not have access to
// these env vars, such as kube-proxy, which sets up the API server endpoint
// (chicken and egg), and the checkpointer, which needs to run as a static pod
// even if the API server isn't available.
var kubeConfigInClusterTemplate = []byte(`apiVersion: v1
kind: ConfigMap
metadata:
  name: kubeconfig-in-cluster
  namespace: kube-system
data:
  kubeconfig: |
    apiVersion: v1
    clusters:
    - name: local
      cluster:
        server: {{ .Server }}
        certificate-authority: /var/run/secrets/kubernetes.io/serviceaccount/ca.crt
    users:
    - name: service-account
      user:
        # Use service account token
        tokenFile: /var/run/secrets/kubernetes.io/serviceaccount/token
    contexts:
    - context:
        cluster: local
        user: service-account
`)

var coreDNSTemplate = []byte(`apiVersion: v1
kind: ServiceAccount
metadata:
  name: coredns
  namespace: kube-system
  labels:
    kubernetes.io/cluster-service: "true"
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRoleBinding
metadata:
  name: system:coredns
  labels:
    kubernetes.io/bootstrapping: rbac-defaults
  annotations:
    rbac.authorization.kubernetes.io/autoupdate: "true"
roleRef:
  apiGroup: rbac.authorization.k8s.io
  kind: ClusterRole
  name: system:coredns
subjects:
  - kind: ServiceAccount
    name: coredns
    namespace: kube-system
---
apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
  name: system:coredns
  labels:
    kubernetes.io/bootstrapping: rbac-defaults
rules:
  - apiGroups: [""]
    resources:
      - endpoints
      - services
      - pods
      - namespaces
    verbs:
      - list
      - watch
  - apiGroups: [""]
    resources:
      - nodes
    verbs:
      - get
  - apiGroups: ["discovery.k8s.io"]
    resources:
      - endpointslices
    verbs:
      - list
      - watch
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: coredns
  namespace: kube-system
data:
  Corefile: |
    .:53 {
        errors
        health {
            lameduck 5s
        }
        ready
        log . {
            class error
        }
        prometheus :9153

        kubernetes {{ .ClusterDomain }} in-addr.arpa ip6.arpa {
            pods insecure
            fallthrough in-addr.arpa ip6.arpa
        }
        forward . /etc/resolv.conf
        cache 30
        loop
        reload
        loadbalance
    }
---
apiVersion: apps/v1
kind: Deployment
metadata:
  name: coredns
  namespace: kube-system
  labels:
    k8s-app: kube-dns
    kubernetes.io/name: "CoreDNS"
    kubernetes.io/cluster-service: "true"
spec:
  replicas: 2
  strategy:
    type: RollingUpdate
    rollingUpdate:
      maxUnavailable: 1
  selector:
    matchLabels:
      k8s-app: kube-dns
  template:
    metadata:
      labels:
        k8s-app: kube-dns
    spec:
      affinity:
        podAntiAffinity:
          preferredDuringSchedulingIgnoredDuringExecution:
          - weight: 100
            podAffinityTerm:
              labelSelector:
                matchExpressions:
                - key: k8s-app
                  operator: In
                  values:
                  - kube-dns
              topologyKey: kubernetes.io/hostname
      serviceAccountName: coredns
      priorityClassName: system-cluster-critical
      tolerations:
        - key: node-role.kubernetes.io/master
          operator: Exists
          effect: NoSchedule
        - key: node-role.kubernetes.io/control-plane
          operator: Exists
          effect: NoSchedule
      containers:
        - name: coredns
          image: {{ .CoreDNSImage }}
          imagePullPolicy: IfNotPresent
          resources:
            limits:
              memory: 170Mi
            requests:
              cpu: 100m
              memory: 70Mi
          args: [ "-conf", "/etc/coredns/Corefile" ]
          volumeMounts:
            - name: config-volume
              mountPath: /etc/coredns
              readOnly: true
          ports:
            - name: dns
              protocol: UDP
              containerPort: 53
            - name: dns-tcp
              protocol: TCP
              containerPort: 53
            - name: metrics
              protocol: TCP
              containerPort: 9153
          livenessProbe:
            httpGet:
              path: /health
              port: 8080
              scheme: HTTP
            initialDelaySeconds: 60
            timeoutSeconds: 5
            successThreshold: 1
            failureThreshold: 5
          readinessProbe:
            httpGet:
              path: /ready
              port: 8181
              scheme: HTTP
          securityContext:
            allowPrivilegeEscalation: false
            capabilities:
              add:
              - NET_BIND_SERVICE
              drop:
              - all
            readOnlyRootFilesystem: true
      dnsPolicy: Default
      volumes:
        - name: config-volume
          configMap:
            name: coredns
            items:
            - key: Corefile
              path: Corefile
`)

var coreDNSSvcTemplate = []byte(`apiVersion: v1
kind: Service
metadata:
  name: kube-dns
  namespace: kube-system
  annotations:
    prometheus.io/scrape: "true"
    prometheus.io/port: "9153"
  labels:
    k8s-app: kube-dns
    kubernetes.io/cluster-service: "true"
    kubernetes.io/name: "CoreDNS"
spec:
  selector:
    k8s-app: kube-dns
  clusterIP: {{ or .DNSServiceIP .DNSServiceIPv6 }}
  clusterIPs:
  {{- if .DNSServiceIP }}
    - {{ .DNSServiceIP }}
  {{- end }}
  {{- if .DNSServiceIPv6 }}
    - {{ .DNSServiceIPv6 }}
  {{- end }}
  ipFamilies:
  {{- if .DNSServiceIP }}
    - IPv4
  {{- end }}
  {{- if .DNSServiceIPv6 }}
    - IPv6
  {{- end }}
  {{- if and .DNSServiceIP .DNSServiceIPv6 }}
  ipFamilyPolicy: RequireDualStack
  {{- else }}
  ipFamilyPolicy: SingleStack
  {{- end }}
  ports:
    - name: dns
      port: 53
      protocol: UDP
    - name: dns-tcp
      port: 53
      protocol: TCP
`)

var flannelTemplate = []byte(`apiVersion: rbac.authorization.k8s.io/v1
kind: ClusterRole
metadata:
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
      "cniVersion": "0.3.1",
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
      "Network": "{{ index .PodCIDRs 0 }}",
      "Backend": {
        "Type": "vxlan",
        "Port": 4789
      }
    }
---
apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: kube-flannel
  namespace: kube-system
  labels:
    tier: node
    k8s-app: flannel
spec:
  selector:
    matchLabels:
      tier: node
      k8s-app: flannel
  template:
    metadata:
      labels:
        tier: node
        k8s-app: flannel
    spec:
      serviceAccountName: flannel
      initContainers:
      - name: install-config
        image: {{ .FlannelImage }}
        command:
        - cp
        args:
        - -f
        - /etc/kube-flannel/cni-conf.json
        - /etc/cni/net.d/10-flannel.conflist
        volumeMounts:
        - name: cni
          mountPath: /etc/cni/net.d
        - name: flannel-cfg
          mountPath: /etc/kube-flannel/
      - name: install-cni
        image: {{ .FlannelCNIImage }}
        command: ["/install-cni.sh"]
        volumeMounts:
        - name: host-cni-bin
          mountPath: /host/opt/cni/bin/
      containers:
      - name: kube-flannel
        image: {{ .FlannelImage }}
        command:
        - /opt/bin/flanneld
        args:
        - --ip-masq
        - --kube-subnet-mgr
        securityContext:
          privileged: true
          capabilities:
            add: ["NET_ADMIN", "NET_RAW"]
        env:
        - name: POD_NAME
          valueFrom:
            fieldRef:
              fieldPath: metadata.name
        - name: POD_NAMESPACE
          valueFrom:
            fieldRef:
              fieldPath: metadata.namespace
        - name: POD_IP
          valueFrom:
            fieldRef:
              fieldPath: status.podIP
        volumeMounts:
        - name: run
          mountPath: /run/flannel
        - name: flannel-cfg
          mountPath: /etc/kube-flannel/
      hostNetwork: true
      tolerations:
      - effect: NoSchedule
        operator: Exists
      - effect: NoExecute
        operator: Exists
      volumes:
        - name: run
          hostPath:
            path: /run/flannel
        - name: cni
          hostPath:
            path: /etc/cni/net.d
        - name: flannel-cfg
          configMap:
            name: kube-flannel-cfg
        - name: host-cni-bin
          hostPath:
            path: /opt/cni/bin
  updateStrategy:
    rollingUpdate:
      maxUnavailable: 1
    type: RollingUpdate
`)

// podSecurityPolicy is the default PSP.
var podSecurityPolicy = []byte(`kind: ClusterRole
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: psp:privileged
rules:
- apiGroups: ['policy']
  resources: ['podsecuritypolicies']
  verbs:     ['use']
  resourceNames:
  - privileged
---
kind: ClusterRoleBinding
apiVersion: rbac.authorization.k8s.io/v1
metadata:
  name: psp:privileged
roleRef:
  kind: ClusterRole
  name: psp:privileged
  apiGroup: rbac.authorization.k8s.io
subjects:
# Authorize all service accounts in a namespace:
- kind: Group
  apiGroup: rbac.authorization.k8s.io
  name: system:serviceaccounts
# Authorize all authenticated users in a namespace:
- kind: Group
  apiGroup: rbac.authorization.k8s.io
  name: system:authenticated
---
apiVersion: policy/v1beta1
kind: PodSecurityPolicy
metadata:
  name: privileged
  annotations:
    seccomp.security.alpha.kubernetes.io/allowedProfileNames: '*'
spec:
  fsGroup:
    rule: RunAsAny
  privileged: true
  runAsUser:
    rule: RunAsAny
  seLinux:
    rule: RunAsAny
  supplementalGroups:
    rule: RunAsAny
  volumes:
  - '*'
  allowedCapabilities:
  - '*'
  hostPID: true
  hostIPC: true
  hostNetwork: true
  hostPorts:
  - min: 1
    max: 65536
`)
