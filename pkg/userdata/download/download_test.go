/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package download

import (
	"encoding/base64"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
)

type downloadSuite struct {
	suite.Suite
}

func TestDownloadSuite(t *testing.T) {
	suite.Run(t, new(downloadSuite))
}

// nolint: dupl
func (suite *downloadSuite) TestV0Download() {
	// Disable logging for test
	log.SetOutput(ioutil.Discard)
	ts := testUDServer()
	defer ts.Close()

	var err error

	// Download plain-text string
	_, err = Download(ts.URL, WithMaxWait(0.1), WithHeaders(map[string]string{"configVersion": "v0"}))
	suite.Require().NoError(err)

	// Download b64 string
	_, err = Download(
		ts.URL,
		WithFormat(b64),
		WithRetries(1),
		WithHeaders(map[string]string{"Metadata": "true", "format": b64, "configVersion": "v0"}),
	)
	suite.Require().NoError(err)
	log.SetOutput(os.Stderr)
}

// nolint: dupl
func (suite *downloadSuite) TestV1Download() {
	// Disable logging for test
	log.SetOutput(ioutil.Discard)
	ts := testUDServer()
	defer ts.Close()

	var err error

	_, err = Download(ts.URL, WithMaxWait(0.1), WithHeaders(map[string]string{"configVersion": "v1"}))
	suite.Require().NoError(err)

	_, err = Download(
		ts.URL,
		WithFormat(b64),
		WithRetries(1),
		WithHeaders(map[string]string{"Metadata": "true", "format": b64, "configVersion": "v1"}),
	)
	suite.Require().NoError(err)
	log.SetOutput(os.Stderr)
}

func testUDServer() *httptest.Server {
	var count int

	testMap := map[string]string{
		"v0": testV0Config,
		"v1": testV1Config,
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		log.Printf("Request %d\n", count)
		if count < 2 {
			w.WriteHeader(http.StatusInternalServerError)
		}

		if r.Header.Get("format") == b64 {
			// nolint: errcheck
			w.Write([]byte(base64.StdEncoding.EncodeToString([]byte(testMap[r.Header.Get("configVersion")]))))
		} else {
			// nolint: errcheck
			w.Write([]byte(testMap[r.Header.Get("configVersion")]))
		}
	}))

	return ts
}

// nolint: lll
const testV1Config = `version: v1
machine:
  type: init
  token: 57dn7x.k5jc6dum97cotlqb
  ca:
    crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCi0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0=
    key: LS0tLS1CRUdJTiBFQyBQUklWQVRFIEtFWS0tLS0tCi0tLS0tRU5EIEVDIFBSSVZBVEUgS0VZLS0tLS0=
  kubelet: {}
  network: {}
  install: {}
cluster:
  controlPlane:
    ips:
    - 10.254.0.10
  clusterName: spencer-test
  network:
    dnsDomain: cluster.local
    podSubnets:
    - 10.244.0.0/16
    serviceSubnets:
    - 10.96.0.0/12
  token: 4iysc6.t3bsjbrd74v91wpv
  initToken: 22c11be4-c413-11e9-b8e8-309c23e4bd47
  ca:
    crt: LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCi0tLS0tRU5EIENFUlRJRklDQVRFLS0tLS0=
    key: LS0tLS1CRUdJTiBFQyBQUklWQVRFIEtFWS0tLS0tCi0tLS0tRU5EIEVDIFBSSVZBVEUgS0VZLS0tLS0=
  apiServer: {}
  controllerManager: {}
  scheduler: {}
  etcd: {}
`

// nolint: lll
const testV0Config = `version: ""
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
      apiVersion: kubeadm.k8s.io/v1beta2
      kind: InitConfiguration
      localAPIEndpoint:
        bindPort: 6443
      bootstrapTokens:
      - token: '1qbsj9.3oz5hsk6grdfp98b'
        ttl: 0s
      ---
      apiVersion: kubeadm.k8s.io/v1beta2
      kind: ClusterConfiguration
      clusterName: test
      kubernetesVersion: v1.16.0-alpha.3
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
  ephemeral:
    force: true
    device: /dev/sda
    size: 1024000000
`
