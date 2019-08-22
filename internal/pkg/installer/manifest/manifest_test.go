/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package manifest

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/talos-systems/talos/pkg/userdata"
	"gopkg.in/yaml.v2"
)

type manifestSuite struct {
	suite.Suite
}

func TestManifestSuite(t *testing.T) {
	suite.Run(t, new(manifestSuite))
}

func (suite *manifestSuite) TestNewManifest() {
	// Test with whole data
	data := &userdata.UserData{}
	err := yaml.Unmarshal([]byte(testConfig), data)
	suite.Require().NoError(err)

	manifests, err := NewManifest(data)
	suite.Require().NoError(err)
	assert.Equal(suite.T(), 2, len(manifests.Targets["/dev/sda"]))
}

func (suite *manifestSuite) TestTargetInstall() {
	// Create Temp dirname for mountpoint
	dir, err := ioutil.TempDir("", "talostest")
	suite.Require().NoError(err)

	// nolint: errcheck
	defer os.RemoveAll(dir)

	// Create a tempfile for local copy
	tempfile, err := ioutil.TempFile("", "example")
	suite.Require().NoError(err)

	// nolint: errcheck
	defer os.Remove(dir)

	// Create simple http test server to serve up some content
	mux := http.NewServeMux()
	mux.HandleFunc("/yolo", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// nolint: errcheck
		w.Write([]byte(testConfig))
	}))
	ts := httptest.NewServer(mux)

	defer ts.Close()

	// Attempt to download and copy files
	target := &Target{
		MountPoint: dir,
		Assets: []*Asset{
			{
				Source:      "file://" + tempfile.Name(),
				Destination: "/path/relative/to/mountpoint/example",
			},
			{
				Source:      ts.URL + "/yolo",
				Destination: "/path/relative/to/mountpoint/yolo",
			},
		},
	}

	suite.Require().NoError(target.Save())

	for _, expectedFile := range target.Assets {
		// Verify downloaded/copied file is at the appropriate location
		_, err := os.Stat(filepath.Join(target.MountPoint, expectedFile.Destination))
		suite.Require().NoError(err)
	}
}

// TODO we should move this to a well defined location
// Copied from userdata_test.go
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
    device: /dev/sda
    size: 1024000000
  ephemeral:
    device: /dev/sda
    size: 1024000000
`
