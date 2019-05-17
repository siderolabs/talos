/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package userdata

import (
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
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

func (suite *validateSuite) TestDownloadRetry() {
	// Disable logging for test
	log.SetOutput(ioutil.Discard)
	ts := testUDServer()
	defer ts.Close()

	_, err := Download(ts.URL, nil)
	suite.Require().NoError(err)
	log.SetOutput(os.Stderr)
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

func testUDServer() *httptest.Server {
	var count int

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		log.Printf("Request %d\n", count)
		if count == 3 {
			// nolint: errcheck
			w.Write([]byte(testConfig))
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
	}))

	return ts
}

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
    initToken: 528d1ad6-3485-49ad-94cd-0f44a35877ac
    certificateKey: 'test'
    configuration: |
      apiVersion: kubeadm.k8s.io/v1beta1
      kind: InitConfiguration
      localAPIEndpoint:
        bindPort: 6443
      bootstrapTokens:
      - token: '1qbsj9.3oz5hsk6grdfp98b'
        ttl: 0s
      ---
      apiVersion: kubeadm.k8s.io/v1beta1
      kind: ClusterConfiguration
      clusterName: test
      kubernetesVersion: v1.14.1
      ---
      apiVersion: kubeproxy.config.k8s.io/v1alpha1
      kind: KubeProxyConfiguration
      mode: ipvs
      ipvs:
        scheduler: lc
  trustd:
    username: 'test'
    password: 'test'
    endpoints: [ "1.2.3.4" ]
    certSANs: []
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

// nolint: lll
const kubeadmConfig = `configuration: |
  apiVersion: kubeadm.k8s.io/v1beta1
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
  apiVersion: kubeadm.k8s.io/v1beta1
  certificatesDir: /etc/kubernetes/pki
  clusterName: test
  controlPlaneEndpoint: ""
  controllerManager: {}
  dns:
    type: CoreDNS
  etcd:
    local:
      dataDir: /var/lib/etcd
  imageRepository: k8s.gcr.io
  kind: ClusterConfiguration
  kubernetesVersion: v1.14.1
  networking:
    dnsDomain: cluster.local
    podSubnet: ""
    serviceSubnet: 10.96.0.0/12
  scheduler: {}
  ---
  apiVersion: kubeproxy.config.k8s.io/v1alpha1
  bindAddress: 0.0.0.0
  clientConnection:
    acceptContentTypes: ""
    burst: 10
    contentType: application/vnd.kubernetes.protobuf
    kubeconfig: /var/lib/kube-proxy/kubeconfig.conf
    qps: 5
  clusterCIDR: ""
  configSyncPeriod: 15m0s
  conntrack:
    max: null
    maxPerCore: 32768
    min: 131072
    tcpCloseWaitTimeout: 1h0m0s
    tcpEstablishedTimeout: 24h0m0s
  enableProfiling: false
  healthzBindAddress: 0.0.0.0:10256
  hostnameOverride: ""
  iptables:
    masqueradeAll: false
    masqueradeBit: 14
    minSyncPeriod: 0s
    syncPeriod: 30s
  ipvs:
    excludeCIDRs: null
    minSyncPeriod: 0s
    scheduler: lc
    strictARP: false
    syncPeriod: 30s
  kind: KubeProxyConfiguration
  metricsBindAddress: 127.0.0.1:10249
  mode: ipvs
  nodePortAddresses: null
  oomScoreAdj: -999
  portRange: ""
  resourceContainer: /kube-proxy
  udpIdleTimeout: 250ms
  winkernel:
    enableDSR: false
    networkName: ""
    sourceVip: ""
certificateKey: test
initToken: 528d1ad6-3485-49ad-94cd-0f44a35877ac
`
