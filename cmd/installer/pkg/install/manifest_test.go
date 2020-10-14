// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package install_test

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/go-blockdevice/blockdevice"

	"github.com/talos-systems/talos/cmd/installer/pkg/install"
	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
	"github.com/talos-systems/talos/internal/pkg/loopback"
)

// Some tests in this package cannot be run under buildkit, as buildkit doesn't propagate partition devices
// like /dev/loopXpY into the sandbox. To run the tests on your local computer, do the following:
//
//	 sudo go test -v --count 1 ./cmd/installer/pkg/install/

type manifestSuite struct {
	suite.Suite

	disk           *os.File
	loopbackDevice *os.File
}

const diskSize = 10 * 1024 * 1024 * 1024 * 1024 // 10 GiB

func TestManifestSuite(t *testing.T) {
	suite.Run(t, new(manifestSuite))
}

func (suite *manifestSuite) SetupSuite() {
	var err error

	suite.disk, err = ioutil.TempFile("", "talos")
	suite.Require().NoError(err)

	suite.Require().NoError(suite.disk.Truncate(diskSize))

	suite.loopbackDevice, err = loopback.NextLoopDevice()
	suite.Require().NoError(err)

	suite.T().Logf("Using %s", suite.loopbackDevice.Name())

	suite.Require().NoError(loopback.Loop(suite.loopbackDevice, suite.disk))

	suite.Require().NoError(loopback.LoopSetReadWrite(suite.loopbackDevice))
}

func (suite *manifestSuite) TearDownSuite() {
	if suite.loopbackDevice != nil {
		suite.Assert().NoError(loopback.Unloop(suite.loopbackDevice))
	}

	if suite.disk != nil {
		suite.Assert().NoError(os.Remove(suite.disk.Name()))
		suite.Assert().NoError(suite.disk.Close())
	}
}

func (suite *manifestSuite) skipUnderBuildkit() {
	hostname, _ := os.Hostname() //nolint: errcheck

	if hostname == "buildkitsandbox" {
		suite.T().Skip("test not supported under buildkit as partition devices are not propagated from /dev")
	}
}

func (suite *manifestSuite) verifyBlockdevice() {
	bd, err := blockdevice.Open(suite.loopbackDevice.Name())
	suite.Require().NoError(err)

	defer bd.Close() //nolint: errcheck

	table, err := bd.PartitionTable()
	suite.Require().NoError(err)

	suite.Assert().Len(table.Partitions(), 5)

	suite.Assert().NoError(bd.Close())
}

func (suite *manifestSuite) TestExecuteManifestClean() {
	suite.skipUnderBuildkit()

	manifest, err := install.NewManifest("A", runtime.SequenceInstall, &install.Options{
		Disk:  suite.loopbackDevice.Name(),
		Force: true,
	})
	suite.Require().NoError(err)

	suite.Assert().NoError(manifest.ExecuteManifest())

	suite.verifyBlockdevice()
}

func (suite *manifestSuite) TestTargetInstall() {
	// Create Temp dirname for mountpoint
	dir, err := ioutil.TempDir("", "talostest")
	suite.Require().NoError(err)

	// nolint: errcheck
	defer os.RemoveAll(dir)

	// Create a tempfile for local copy
	tempfile, err := ioutil.TempFile(dir, "example")
	suite.Require().NoError(err)

	// Create simple http test server to serve up some content
	mux := http.NewServeMux()
	mux.HandleFunc("/yolo", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// nolint: errcheck
		w.Write([]byte("null"))
	}))

	ts := httptest.NewServer(mux)

	defer ts.Close()
	// Attempt to download and copy files
	target := &install.Target{
		Assets: []*install.Asset{
			{
				Source:      tempfile.Name(),
				Destination: "/path/relative/to/mountpoint/example",
			},
		},
	}

	suite.Require().NoError(target.Save())

	for _, expectedFile := range target.Assets {
		// Verify copied file is at the appropriate location.
		_, err := os.Stat(expectedFile.Destination)
		suite.Require().NoError(err)
	}
}
