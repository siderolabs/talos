/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package generate

const initTempl = `---
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
      localAPIEndpoint:
        bindPort: 6443
      bootstrapTokens:
      - token: '{{ .KubeadmTokens.BootstrapToken }}'
        ttl: 0s
      nodeRegistration:
        taints: []
        kubeletExtraArgs:
          node-labels: ""
      ---
      apiVersion: kubeadm.k8s.io/v1beta1
      kind: ClusterConfiguration
      clusterName: {{ .ClusterName }}
      kubernetesVersion: {{ .KubernetesVersion }}
      controlPlaneEndpoint: {{ index .MasterIPs 0 }}:443
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
    certSANs: [ "{{ index .MasterIPs .Index }}", "127.0.0.1" ]
`
