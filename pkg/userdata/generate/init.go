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
    certificateKey: '{{ .KubeadmTokens.CertKey }}'
    configuration: |
      apiVersion: kubeadm.k8s.io/v1beta1
      kind: InitConfiguration
      apiEndpoint:
        advertiseAddress: {{ index .MasterIPs .Index }}
        bindPort: 6443
      bootstrapTokens:
      - token: '{{ .KubeadmTokens.BootstrapToken }}'
        ttl: 0s
      nodeRegistration:
        taints: []
        kubeletExtraArgs:
          node-labels: ""
          feature-gates: ExperimentalCriticalPodAnnotation=true
      ---
      apiVersion: kubeadm.k8s.io/v1beta1
      kind: ClusterConfiguration
      clusterName: {{ .ClusterName }}
      controlPlaneEndpoint: {{ index .MasterIPs 0 }}
      apiServer:
        certSANs: [ {{ range $i,$ip := .MasterIPs }}{{if $i}},{{end}}"{{$ip}}"{{end}} ]
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
      apiVersion: kubeproxy.config.k8s.io/v1alpha1
      kind: KubeProxyConfiguration
      mode: ipvs
      ipvs:
        scheduler: lc
  trustd:
    username: '{{ .TrustdInfo.Username }}'
    password: '{{ .TrustdInfo.Password }}'
    endpoints: [ {{ .Endpoints }} ]
    certSANs: [ "{{ index .MasterIPs .Index }}" ]
`
