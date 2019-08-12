/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package generate

const controlPlaneTempl = `#!talos
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
      kind: JoinConfiguration
      controlPlane: {}
      discovery:
        bootstrapToken:
          token: '{{ .KubeadmTokens.BootstrapToken }}'
          unsafeSkipCAVerification: true
          apiServerEndpoint: "{{ .GetControlPlaneEndpoint "6443" }}"
      nodeRegistration:
        taints: []
        kubeletExtraArgs:
          node-labels: ""
          feature-gates: ExperimentalCriticalPodAnnotation=true
  trustd:
    token: '{{ .TrustdInfo.Token }}'
    endpoints: [ {{ .Endpoints }} ]
    certSANs: [ "{{ .IP }}" ]
`
