/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package userdata

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	yaml "gopkg.in/yaml.v2"
)

type validateSuite struct {
	suite.Suite
}

func TestValidateSuite(t *testing.T) {
	suite.Run(t, new(validateSuite))
}

func (suite *validateSuite) TestKubeadmMarshal() {
	var kubeadm Kubeadm

	err := yaml.Unmarshal([]byte(kubeadmConfig), &kubeadm)
	suite.Require().NoError(err)

	assert.Equal(suite.T(), "test", kubeadm.CertificateKey)

	out, err := yaml.Marshal(&kubeadm)
	suite.Require().NoError(err)

	assert.Equal(suite.T(), kubeadmConfig, string(out))
}

// nolint: lll
const kubeadmConfig = `configuration: |
  apiVersion: kubeadm.k8s.io/v1beta2
  bootstrapTokens:
  - groups:
    - system:bootstrappers:kubeadm:default-node-token
    token: 1qbsj9.3oz5hsk6grdfp98b
    ttl: 0s
    usages:
    - signing
    - authentication
  kind: InitConfiguration
  localAPIEndpoint:
    advertiseAddress: 192.168.88.11
    bindPort: 6443
  nodeRegistration:
    criSocket: /var/run/dockershim.sock
    name: smiradell
    taints:
    - effect: NoSchedule
      key: node-role.kubernetes.io/master
  ---
  apiServer:
    timeoutForControlPlane: 4m0s
  apiVersion: kubeadm.k8s.io/v1beta2
  certificatesDir: /etc/kubernetes/pki
  clusterName: test
  controllerManager: {}
  dns:
    type: CoreDNS
  etcd:
    local:
      dataDir: /var/lib/etcd
  imageRepository: k8s.gcr.io
  kind: ClusterConfiguration
  kubernetesVersion: v1.16.0-alpha.3
  networking:
    dnsDomain: cluster.local
    serviceSubnet: 10.96.0.0/12
  scheduler: {}
certificateKey: test
initToken: 528d1ad6-3485-49ad-94cd-0f44a35877ac
controlplane: true
`
