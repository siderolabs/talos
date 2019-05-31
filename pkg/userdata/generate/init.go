/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package generate

const initTempl = `#!talos
version: ""
security:
  os:
    ca:
      crt: "{{ .Certs.OsCert }}"
      key: "{{ .Certs.OsKey }}"
  kubernetes:
    ca:
      crt: "{{ .Certs.K8sCert }}"
      key: "{{ .Certs.K8sKey }}"
services:
  init:
    cni: flannel
  kubeadm:
    initToken: {{ .InitToken }}
    certificateKey: '{{ .KubeadmTokens.CertKey }}'
    configuration: |
      apiVersion: kubeadm.k8s.io/v1beta1
      kind: InitConfiguration
      bootstrapTokens:
      - token: '{{ .KubeadmTokens.BootstrapToken }}'
        ttl: 0s
      localAPIEndpoint:
        bindPort: 6443
      nodeRegistration:
        criSocket: /run/containerd/containerd.sock
      ---
      apiVersion: kubeadm.k8s.io/v1beta1
      kind: ClusterConfiguration
      clusterName: {{ .ClusterName }}
      kubernetesVersion: {{ .KubernetesVersion }}
      controlPlaneEndpoint: {{ .IP }}:443
      apiServer:
        certSANs: [ {{ range $i,$ip := .MasterIPs }}{{if $i}},{{end}}"{{$ip}}"{{end}}, "127.0.0.1" ]
        extraArgs:
          runtime-config: settings.k8s.io/v1alpha1=true
          feature-gates: ExperimentalCriticalPodAnnotation=true
      controllerManager:
        extraArgs:
          terminated-pod-gc-threshold: '100'
          feature-gates: ExperimentalCriticalPodAnnotation=true
      scheduler:
        extraArgs:
          feature-gates: ExperimentalCriticalPodAnnotation=true
      networking:
        dnsDomain: {{ .ServiceDomain }}
        podSubnet: {{ index .PodNet 0 }}
        serviceSubnet: {{ index .ServiceNet 0 }}
      etcd:
        local:
          serverCertSANs:
            - master-{{ .Index }}
            - {{ .IP }}
          peerCertSANs:
            - master-{{ .Index }}
            - {{ .IP }}
          extraArgs:
            initial-cluster: {{ range $i,$ip := .MasterIPs }}{{if $i}},{{end}}master-{{add $i 1}}=https://{{$ip}}:2380{{end}}
            initial-cluster-state: new
            listen-peer-urls: https://{{ .IP }}:2380
            listen-client-urls: https://{{ .IP }}:2379
            advertise-client-urls: https://{{ .IP }}:2379
            initial-advertise-peer-urls: https://{{ .IP }}:2380
      ---
      apiVersion: kubelet.config.k8s.io/v1beta1
      kind: KubeletConfiguration
      featureGates:
        ExperimentalCriticalPodAnnotation: true
      ---
      apiVersion: kubeproxy.config.k8s.io/v1alpha1
      kind: KubeProxyConfiguration
      mode: ipvs
      ipvs:
        scheduler: lc
  trustd:
    token: '{{ .TrustdInfo.Token }}'
    endpoints: [ {{ .Endpoints }} ]
    certSANs: [ "{{ .IP }}", "127.0.0.1" ]
`
