/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package manifest

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/stretchr/testify/suite"
)

type manifestSuite struct {
	suite.Suite
}

func TestManifestSuite(t *testing.T) {
	suite.Run(t, new(manifestSuite))
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
	target := &Target{
		Assets: []*Asset{
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
