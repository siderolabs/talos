/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package generate_test

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/talos/pkg/userdata/generate"
)

const (
	expectedInitConfig = `---
version: ""
security:
  os:
    ca:
      crt: ""
      key: ""
  kubernetes:
    ca:
      crt: ""
      key: ""
services:
  init:
    cni: flannel
  kubeadm:
    certificateKey: 'testcrtkey'
    configuration: |
      apiVersion: kubeadm.k8s.io/v1beta1
      kind: InitConfiguration
      apiEndpoint:
        advertiseAddress: 10.0.1.5
        bindPort: 6443
      bootstrapTokens:
      - token: 'abcdef.1234567890123456789'
        ttl: 0s
      nodeRegistration:
        taints: []
        kubeletExtraArgs:
          node-labels: ""
          feature-gates: ExperimentalCriticalPodAnnotation=true
      ---
      apiVersion: kubeadm.k8s.io/v1beta1
      kind: ClusterConfiguration
      clusterName: test
      controlPlaneEndpoint: 10.0.1.5
      apiServer:
        certSANs: [ "10.0.1.5","10.0.1.6","10.0.1.7" ]
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
        dnsDomain: cluster.local
        podSubnet: 10.244.0.0/16
        serviceSubnet: 10.96.0.0/12
      ---
      apiVersion: kubeproxy.config.k8s.io/v1alpha1
      kind: KubeProxyConfiguration
      mode: ipvs
      ipvs:
        scheduler: lc
  trustd:
    username: 'test'
    password: 'test'
    endpoints: [  ]
    certSANs: [ "10.0.1.5" ]
`

	expectedControlPlaneConfig = `---
version: ""
security: null
services:
  init:
    cni: flannel
  kubeadm:
    certificateKey: 'testcrtkey'
    configuration: |
      apiVersion: kubeadm.k8s.io/v1beta1
      kind: JoinConfiguration
      controlPlane:
        apiEndpoint:
          advertiseAddress: 10.0.1.5
          bindPort: 6443
      discovery:
        bootstrapToken:
          token: 'abcdef.1234567890123456789'
          unsafeSkipCAVerification: true
          apiServerEndpoint: 10.0.1.5:443
      nodeRegistration:
        taints: []
        kubeletExtraArgs:
          node-labels: ""
          feature-gates: ExperimentalCriticalPodAnnotation=true
  trustd:
    username: 'test'
    password: 'test'
    endpoints: [ "10.0.1.5" ]
    bootstrapNode: "10.0.1.5"
    certSANs: [ "10.0.1.5" ]
`

	expectedWorkerConfig = `---
version: ""
security: null
services:
  init:
    cni: flannel
  kubeadm:
    configuration: |
      apiVersion: kubeadm.k8s.io/v1beta1
      kind: JoinConfiguration
      discovery:
        bootstrapToken:
          token: 'abcdef.1234567890123456789'
          unsafeSkipCAVerification: true
          apiServerEndpoint: 10.0.1.5:443
      nodeRegistration:
        taints: []
        kubeletExtraArgs:
          node-labels: ""
          feature-gates: ExperimentalCriticalPodAnnotation=true
      token: 'abcdef.1234567890123456789'
  trustd:
    username: 'test'
    password: 'test'
    endpoints: [ "10.0.1.5","10.0.1.6","10.0.1.7" ]
`
)

var (
	input = generate.Input{
		Certs:         &generate.Certs{},
		MasterIPs:     []string{"10.0.1.5", "10.0.1.6", "10.0.1.7"},
		PodNet:        []string{"10.244.0.0/16"},
		ServiceNet:    []string{"10.96.0.0/12"},
		ServiceDomain: "cluster.local",
		ClusterName:   "test",
		KubeadmTokens: &generate.KubeadmTokens{
			BootstrapToken: "abcdef.1234567890123456789",
			CertKey:        "testcrtkey",
		},
		TrustdInfo: &generate.TrustdInfo{
			Username: "test",
			Password: "test",
		},
	}
)

type GenerateSuite struct {
	suite.Suite
}

func TestGenerateSuite(t *testing.T) {
	suite.Run(t, new(GenerateSuite))
}

func (suite *GenerateSuite) TestGenerateInitSuccess() {
	i := input
	i.Type = "init"
	userdata, err := generate.Userdata(&i)
	suite.Require().NoError(err)
	suite.Assert().Equal(userdata, expectedInitConfig)
}

func (suite *GenerateSuite) TestGenerateControlPlaneSuccess() {
	i := input
	i.Type = "controlplane"
	userdata, err := generate.Userdata(&i)
	suite.Require().NoError(err)
	suite.Assert().Equal(userdata, expectedControlPlaneConfig)
}

func (suite *GenerateSuite) TestGenerateWorkerSuccess() {
	i := input
	i.Type = "worker"
	userdata, err := generate.Userdata(&i)
	suite.Require().NoError(err)
	suite.Assert().Equal(userdata, expectedWorkerConfig)
}
