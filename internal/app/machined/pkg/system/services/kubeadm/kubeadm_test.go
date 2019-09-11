/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package kubeadm

import (
	"context"
	"io/ioutil"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
	"gopkg.in/yaml.v2"

	"github.com/talos-systems/talos/internal/app/trustd/proto"
	"github.com/talos-systems/talos/pkg/constants"
	"github.com/talos-systems/talos/pkg/grpc/middleware/auth/basic"
	"github.com/talos-systems/talos/pkg/userdata"
)

type KubeadmSuite struct {
	suite.Suite
}

func TestKubeadmSuite(t *testing.T) {
	suite.Run(t, new(KubeadmSuite))
}

func (suite *KubeadmSuite) TestWritePKIFiles() {
	data := &userdata.UserData{}
	data.Security = &userdata.Security{}
	// Simple test where we dont write out any files.
	// Partially limited in current implementation
	// where file locations are hardcoded
	err := WritePKIFiles(data)
	suite.Assert().Error(err)
	data.Security.Kubernetes = &userdata.KubernetesSecurity{}
	err = WritePKIFiles(data)
	suite.Assert().NoError(err)
}

func genUD() (ud *userdata.UserData, err error) {
	ud = &userdata.UserData{}
	err = yaml.Unmarshal([]byte(testConfig), ud)
	return ud, err
}

func (suite *KubeadmSuite) TestEditJoinConfig() {
	data, err := genUD()
	suite.Assert().NoError(err)

	// testConfig is an initConfig
	err = editJoinConfig(data)
	suite.Assert().Error(err)

	data, err = data.Upgrade()
	suite.Assert().NoError(err)

	err = editJoinConfig(data)
	suite.Assert().NoError(err)
}

func (suite *KubeadmSuite) TestEditInitConfig() {
	// Cant test this atm because we run through cis hardening
	// which automatically generates additional assets in
	// hardcoded locations
	data, err := genUD()
	suite.Assert().NoError(err)

	err = editInitConfig(data)
	suite.Assert().NoError(err)

	data, err = data.Upgrade()
	suite.Assert().NoError(err)

	// upgraded config is an joinConfig
	err = editInitConfig(data)
	suite.Assert().Error(err)
}

func (suite *KubeadmSuite) TestFileSet() {
	// Ensure by default we get the expected number of requests
	suite.Assert().Equal(len(FileSet(RequiredFiles())), len(RequiredFiles()))

	// Make sure if local file exists we dont copy it
	tmpfile, err := ioutil.TempFile("", "testfileset")
	suite.Assert().NoError(err)

	// nolint: errcheck
	defer os.Remove(tmpfile.Name())

	suite.Assert().Equal(len(FileSet([]string{tmpfile.Name()})), 0)
}

func (suite *KubeadmSuite) TestDownload() {
	ctx, cancel := context.WithCancel(context.Background())
	// Immediately cancel the context, should prevent us from
	// actually doing anything
	cancel()
	conn, err := basic.NewConnection("localhost", constants.TrustdPort, nil)
	suite.Assert().NoError(err)
	data := download(ctx, proto.NewTrustdClient(conn), &proto.ReadFileRequest{Path: ""})
	suite.Assert().Equal(len(data), 0)

	// suite.Assert().NoError(err)
}

func (suite *KubeadmSuite) TestCreateTrustdClients() {
	data, err := genUD()
	suite.Assert().NoError(err)
	var clients []proto.TrustdClient
	clients, err = CreateTrustdClients(data)
	suite.Assert().NoError(err)
	suite.Assert().Equal(len(clients), 2)
}

// TODO: Find a way to have a common shared test userdata struct
// nolint: lll
const testConfig = `version: "1"
security:
  os:
    ca:
      crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCi0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0=
      key: LS0tLS1CRUdJTiBFQyBQUklWQVRFIEtFWS0tLS0tCi0tLS0tRU5EIEVDIFBSSVZBVEUgS0VZLS0tLS0=
    identity:
      crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCi0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0=
      key: LS0tLS1CRUdJTiBFQyBQUklWQVRFIEtFWS0tLS0tCi0tLS0tRU5EIEVDIFBSSVZBVEUgS0VZLS0tLS0=
  kubernetes:
    ca:
      crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCi0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0=
      key: LS0tLS1CRUdJTiBFQyBQUklWQVRFIEtFWS0tLS0tCi0tLS0tRU5EIEVDIFBSSVZBVEUgS0VZLS0tLS0=
    sa:
      crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCi0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0=
      key: LS0tLS1CRUdJTiBFQyBQUklWQVRFIEtFWS0tLS0tCi0tLS0tRU5EIEVDIFBSSVZBVEUgS0VZLS0tLS0=
    frontproxy:
      crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCi0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0=
      key: LS0tLS1CRUdJTiBFQyBQUklWQVRFIEtFWS0tLS0tCi0tLS0tRU5EIEVDIFBSSVZBVEUgS0VZLS0tLS0=
    etcd:
      crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCi0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0=
      key: LS0tLS1CRUdJTiBFQyBQUklWQVRFIEtFWS0tLS0tCi0tLS0tRU5EIEVDIFBSSVZBVEUgS0VZLS0tLS0=
networking:
  os: {}
  kubernetes: {}
services:
  init:
    cni: flannel
  kubeadm:
    certificateKey: 'dnwrwn05vd2lyghvflnk93nwie'
    configuration: |
      apiVersion: kubeadm.k8s.io/v1beta2
      kind: InitConfiguration
      bootstrapTokens:
      - token: 'gcwogs.kflpeg7yievuh1kq'
        ttl: 0s
      nodeRegistration:
        taints: []
        kubeletExtraArgs:
          node-labels: ""
      ---
      apiVersion: kubeadm.k8s.io/v1beta2
      kind: ClusterConfiguration
      clusterName: cluster.local
      kubernetesVersion: v1.16.0-rc.1
      controlPlaneEndpoint: 1.2.3.4:443
      apiServer:
        certSANs: [ "1.2.3.4", "127.0.0.1" ]
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
        dnsDomain: cluster.local
        podSubnet: 10.244.0.0/16
        serviceSubnet: 10.96.0.0/12
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
    token: '9s2vto.p0xd8u1ap61svw8q'
    endpoints: [ 1.2.3.4, 2.3.4.5 ]
    certSANs: [ "1.2.3.4", "127.0.0.1" ]
install:
  wipe: true
  force: true
  boot:
    force: true
    device: /dev/sda
    size: 1024000000
  root:
    force: true
    device: /dev/sda
    size: 1024000000
  data:
    force: true
    device: /dev/sda
    size: 1024000000
`
