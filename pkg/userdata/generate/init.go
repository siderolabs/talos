/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package generate

const initTempl = `#!talos
version: ""
kubernetesVersion: {{ .KubernetesVersion }}
security:
  os:
    ca:
      crt: "{{ .Certs.OsCert }}"
      key: "{{ .Certs.OsKey }}"
  kubernetes:
    ca:
      crt: "{{ .Certs.K8sCert }}"
      key: "{{ .Certs.K8sKey }}"
    aescbcEncryptionSecret: {{ .KubeadmTokens.AESCBCEncryptionSecret }}
  etcd:
    ca:
      crt: "{{ .Certs.EtcdCert }}"
      key: "{{ .Certs.EtcdKey }}"
services:
  init:
    cni: flannel
  kubeadm:
    configuration: |
      apiVersion: kubeadm.k8s.io/v1beta2
      kind: InitConfiguration
      bootstrapTokens:
      - token: '{{ .KubeadmTokens.BootstrapToken }}'
        ttl: 0s
      certificateKey: {{ .KubeadmTokens.CertificateKey }}
      nodeRegistration:
        taints: []
        kubeletExtraArgs:
          node-labels: ""
      ---
      apiVersion: kubeadm.k8s.io/v1beta2
      kind: ClusterConfiguration
      clusterName: {{ .ClusterName }}
      kubernetesVersion: {{ .KubernetesVersion }}
      controlPlaneEndpoint: "{{ .GetControlPlaneEndpoint }}"
      apiServer:
        certSANs: [ {{ range $i,$addr := .GetAPIServerSANs }}{{if $i}},{{end}}"{{$addr}}"{{end}} ]
        extraArgs:
          runtime-config: settings.k8s.io/v1alpha1=true
          feature-gates: ""
      controllerManager:
        extraArgs:
          terminated-pod-gc-threshold: '100'
          feature-gates: ""
      scheduler:
        extraArgs:
          feature-gates: ""
      networking:
        dnsDomain: {{ .ServiceDomain }}
        podSubnet: "{{ index .PodNet 0 }}"
        serviceSubnet: "{{ index .ServiceNet 0 }}"
      ---
      apiVersion: kubelet.config.k8s.io/v1beta1
      kind: KubeletConfiguration
      featureGates: {}
      ---
      apiVersion: kubeproxy.config.k8s.io/v1alpha1
      kind: KubeProxyConfiguration
      mode: ipvs
      ipvs:
        scheduler: lc
  trustd:
    token: '{{ .TrustdInfo.Token }}'
    endpoints: [ {{ .Endpoints }} ]
    certSANs: [ "{{ .IP }}", "127.0.0.1", "::1" ]
`
