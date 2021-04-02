// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_cli

package cli

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"regexp"

	"github.com/talos-systems/talos/internal/integration/base"
	"github.com/talos-systems/talos/pkg/machinery/config/configloader"
)

// GenSuite verifies dmesg command.
type GenSuite struct {
	base.CLISuite

	tmpDir   string
	savedCwd string
}

// SuiteName ...
func (suite *GenSuite) SuiteName() string {
	return "cli.GenSuite"
}

// SetupTest ...
func (suite *GenSuite) SetupTest() {
	var err error
	suite.tmpDir, err = ioutil.TempDir("", "talos")
	suite.Require().NoError(err)

	suite.savedCwd, err = os.Getwd()
	suite.Require().NoError(err)

	suite.Require().NoError(os.Chdir(suite.tmpDir))
}

// TearDownTest ...
func (suite *GenSuite) TearDownTest() {
	if suite.savedCwd != "" {
		suite.Require().NoError(os.Chdir(suite.savedCwd))
	}

	if suite.tmpDir != "" {
		suite.Require().NoError(os.RemoveAll(suite.tmpDir))
	}
}

// TestCA ...
func (suite *GenSuite) TestCA() {
	suite.RunCLI([]string{"gen", "ca", "--organization", "Foo"},
		base.StdoutEmpty())

	suite.Assert().FileExists("Foo.crt")
	suite.Assert().FileExists("Foo.sha256")
	suite.Assert().FileExists("Foo.key")
}

// TestKey ...
func (suite *GenSuite) TestKey() {
	suite.RunCLI([]string{"gen", "key", "--name", "Foo"},
		base.StdoutEmpty())

	suite.Assert().FileExists("Foo.key")
}

// TestCSR ...
func (suite *GenSuite) TestCSR() {
	suite.RunCLI([]string{"gen", "key", "--name", "Foo"},
		base.StdoutEmpty())

	suite.RunCLI([]string{"gen", "csr", "--key", "Foo.key", "--ip", "10.0.0.1"},
		base.StdoutEmpty())

	suite.Assert().FileExists("Foo.csr")
}

// TestCrt ...
func (suite *GenSuite) TestCrt() {
	suite.RunCLI([]string{"gen", "ca", "--organization", "Foo"},
		base.StdoutEmpty())

	suite.RunCLI([]string{"gen", "key", "--name", "Bar"},
		base.StdoutEmpty())

	suite.RunCLI([]string{"gen", "csr", "--key", "Bar.key", "--ip", "10.0.0.1"},
		base.StdoutEmpty())

	suite.RunCLI([]string{"gen", "crt", "--ca", "Foo", "--csr", "Bar.csr", "--name", "foobar"},
		base.StdoutEmpty())

	suite.Assert().FileExists("foobar.crt")
}

// TestKeypair ...
func (suite *GenSuite) TestKeypair() {
	suite.RunCLI([]string{"gen", "keypair", "--organization", "Foo", "--ip", "10.0.0.1"},
		base.StdoutEmpty())

	suite.Assert().FileExists("Foo.crt")
	suite.Assert().FileExists("Foo.key")
}

// TestGenConfigURLValidation ...
func (suite *GenSuite) TestGenConfigURLValidation() {
	suite.RunCLI([]string{"gen", "config", "foo", "192.168.0.1"},
		base.ShouldFail(),
		base.StdoutEmpty(),
		base.StderrNotEmpty(),
		base.StderrShouldMatch(regexp.MustCompile(regexp.QuoteMeta(`try: "https://192.168.0.1:6443"`))))

	suite.RunCLI([]string{"gen", "config", "foo", "192.168.0.1:6443"},
		base.ShouldFail(),
		base.StdoutEmpty(),
		base.StderrNotEmpty(),
		base.StderrShouldMatch(regexp.MustCompile(regexp.QuoteMeta(`try: "https://192.168.0.1:6443"`))))

	suite.RunCLI([]string{"gen", "config", "foo", "192.168.0.1:2000"},
		base.ShouldFail(),
		base.StdoutEmpty(),
		base.StderrNotEmpty(),
		base.StderrShouldMatch(regexp.MustCompile(regexp.QuoteMeta(`try: "https://192.168.0.1:2000"`))))

	suite.RunCLI([]string{"gen", "config", "foo", "http://192.168.0.1:2000"},
		base.ShouldFail(),
		base.StdoutEmpty(),
		base.StderrNotEmpty(),
		base.StderrShouldMatch(regexp.MustCompile(regexp.QuoteMeta(`try: "https://192.168.0.1:2000"`))))
}

// TestGenConfigPatch verify that gen config --config-patch works.
func (suite *GenSuite) TestGenConfigPatch() {
	patch, err := json.Marshal([]map[string]interface{}{
		{
			"op":    "replace",
			"path":  "/cluster/clusterName",
			"value": "bar",
		},
	})

	suite.Assert().NoError(err)

	suite.RunCLI([]string{"gen", "config", "foo", "https://192.168.0.1:6443", "--config-patch", string(patch)})

	cfg, err := configloader.NewFromFile("controlplane.yaml")

	suite.Assert().NoError(err)
	suite.Assert().Equal("bar", cfg.Cluster().Name())
}

func init() {
	allSuites = append(allSuites, new(GenSuite))
}
