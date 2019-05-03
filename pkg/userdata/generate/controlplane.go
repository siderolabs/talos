/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package generate

const controlPlaneTempl = `---
version: ""
security: null
services:
  init:
    cni: flannel
  kubeadm:
    certificateKey: '{{ .KubeadmTokens.CertKey }}'
    configuration: |
      apiVersion: kubeadm.k8s.io/v1beta1
      kind: JoinConfiguration
      controlPlane:
        localAPIEndpoint:
          advertiseAddress: {{ index .MasterIPs .Index }}
          bindPort: 6443
      discovery:
        bootstrapToken:
          token: '{{ .KubeadmTokens.BootstrapToken }}'
          unsafeSkipCAVerification: true
          apiServerEndpoint: {{ index .MasterIPs 0 }}:443
      nodeRegistration:
        taints: []
        kubeletExtraArgs:
          node-labels: ""
          feature-gates: ExperimentalCriticalPodAnnotation=true
  trustd:
    token: '{{ .TrustdInfo.Token }}'
    endpoints: [ "{{ index .MasterIPs 0 }}" ]
    bootstrapNode: "{{ index .MasterIPs 0 }}"
    certSANs: [ "{{ index .MasterIPs .Index }}" ]
`
