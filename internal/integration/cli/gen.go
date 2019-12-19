// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// +build integration_cli

package cli

import (
	"io/ioutil"
	"os"

	"github.com/talos-systems/talos/internal/integration/base"
)

// GenSuite verifies dmesg command
type GenSuite struct {
	base.CLISuite

	tmpDir   string
	savedCwd string
}

// SuiteName ...
func (suite *GenSuite) SuiteName() string {
	return "cli.GenSuite"
}

func (suite *GenSuite) SetupTest() {
	var err error
	suite.tmpDir, err = ioutil.TempDir("", "talos")
	suite.Require().NoError(err)

	suite.savedCwd, err = os.Getwd()
	suite.Require().NoError(err)

	suite.Require().NoError(os.Chdir(suite.tmpDir))
}

func (suite *GenSuite) TearDownTest() {
	suite.Require().NoError(os.Chdir(suite.savedCwd))
	suite.Require().NoError(os.RemoveAll(suite.tmpDir))
}

// TestCA ...
func (suite *GenSuite) TestCA() {
	suite.RunOsctl([]string{"gen", "ca", "--organization", "Foo"},
		base.StdoutEmpty())

	suite.Assert().FileExists("Foo.crt")
	suite.Assert().FileExists("Foo.sha256")
	suite.Assert().FileExists("Foo.key")
}

// TestKey ...
func (suite *GenSuite) TestKey() {
	suite.RunOsctl([]string{"gen", "key", "--name", "Foo"},
		base.StdoutEmpty())

	suite.Assert().FileExists("Foo.key")
}

// TestCSR ...
func (suite *GenSuite) TestCSR() {
	suite.RunOsctl([]string{"gen", "key", "--name", "Foo"},
		base.StdoutEmpty())

	suite.RunOsctl([]string{"gen", "csr", "--key", "Foo.key", "--ip", "10.0.0.1"},
		base.StdoutEmpty())

	suite.Assert().FileExists("Foo.csr")
}

// TestCrt ...
func (suite *GenSuite) TestCrt() {
	suite.RunOsctl([]string{"gen", "ca", "--organization", "Foo"},
		base.StdoutEmpty())

	suite.RunOsctl([]string{"gen", "key", "--name", "Bar"},
		base.StdoutEmpty())

	suite.RunOsctl([]string{"gen", "csr", "--key", "Bar.key", "--ip", "10.0.0.1"},
		base.StdoutEmpty())

	suite.RunOsctl([]string{"gen", "crt", "--ca", "Foo", "--csr", "Bar.csr", "--name", "foobar"},
		base.StdoutEmpty())

	suite.Assert().FileExists("foobar.crt")
}

// TestKeypair ...
func (suite *GenSuite) TestKeypair() {
	suite.RunOsctl([]string{"gen", "keypair", "--organization", "Foo", "--ip", "10.0.0.1"},
		base.StdoutEmpty())

	suite.Assert().FileExists("Foo.crt")
	suite.Assert().FileExists("Foo.key")
}

func init() {
	allSuites = append(allSuites, new(GenSuite))
}
