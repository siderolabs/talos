// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_cli
// +build integration_cli

package cli

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"regexp"

	"gopkg.in/yaml.v3"

	"github.com/talos-systems/talos/internal/integration/base"
	"github.com/talos-systems/talos/pkg/machinery/config/configloader"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/generate"
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
	suite.tmpDir = suite.T().TempDir()

	var err error
	suite.savedCwd, err = os.Getwd()
	suite.Require().NoError(err)

	suite.Require().NoError(os.Chdir(suite.tmpDir))
}

// TearDownTest ...
func (suite *GenSuite) TearDownTest() {
	if suite.savedCwd != "" {
		suite.Require().NoError(os.Chdir(suite.savedCwd))
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
		base.StderrShouldMatch(regexp.MustCompile(`\Qtry: "https://192.168.0.1:6443"`)))

	suite.RunCLI([]string{"gen", "config", "foo", "192.168.0.1:6443"},
		base.ShouldFail(),
		base.StdoutEmpty(),
		base.StderrShouldMatch(regexp.MustCompile(`\Qtry: "https://192.168.0.1:6443"`)))

	suite.RunCLI([]string{"gen", "config", "foo", "192.168.0.1:2000"},
		base.ShouldFail(),
		base.StdoutEmpty(),
		base.StderrShouldMatch(regexp.MustCompile(`\Qtry: "https://192.168.0.1:2000"`)))

	suite.RunCLI([]string{"gen", "config", "foo", "http://192.168.0.1:2000"},
		base.ShouldFail(),
		base.StdoutEmpty(),
		base.StderrShouldMatch(regexp.MustCompile(`\Qtry: "https://192.168.0.1:2000"`)))
}

// TestGenConfigPatchJSON6902 verifies that gen config --config-patch works with JSON patches.
func (suite *GenSuite) TestGenConfigPatchJSON6902() {
	patch, err := json.Marshal([]map[string]interface{}{
		{
			"op":    "replace",
			"path":  "/cluster/clusterName",
			"value": "bar",
		},
	})

	suite.Assert().NoError(err)

	suite.testGenConfigPatch(patch)
}

// TestGenConfigPatchStrategic verifies that gen config --config-patch works with strategic merge patches.
func (suite *GenSuite) TestGenConfigPatchStrategic() {
	patch, err := yaml.Marshal(map[string]interface{}{
		"cluster": map[string]interface{}{
			"clusterName": "bar",
		},
	})

	suite.Assert().NoError(err)

	suite.testGenConfigPatch(patch)
}

func (suite *GenSuite) testGenConfigPatch(patch []byte) {
	for _, tt := range []struct {
		flag         string
		shouldAffect map[string]bool
	}{
		{
			flag: "config-patch",
			shouldAffect: map[string]bool{
				"controlplane.yaml": true,
				"worker.yaml":       true,
			},
		},
		{
			flag: "config-patch-control-plane",
			shouldAffect: map[string]bool{
				"controlplane.yaml": true,
			},
		},
		{
			flag: "config-patch-worker",
			shouldAffect: map[string]bool{
				"worker.yaml": true,
			},
		},
	} {
		tt := tt

		suite.Run(tt.flag, func() {
			suite.RunCLI([]string{"gen", "config", "foo", "https://192.168.0.1:6443", "--" + tt.flag, string(patch)})

			for _, configName := range []string{"controlplane.yaml", "worker.yaml"} {
				cfg, err := configloader.NewFromFile(configName)
				suite.Require().NoError(err)

				switch {
				case tt.shouldAffect[configName]:
					suite.Assert().Equal("bar", cfg.Cluster().Name(), "checking %q", configName)
				case configName == "worker.yaml":
					suite.Assert().Equal("", cfg.Cluster().Name(), "checking %q", configName)
				default:
					suite.Assert().Equal("foo", cfg.Cluster().Name(), "checking %q", configName)
				}
			}
		})
	}
}

// TestSecrets ...
func (suite *GenSuite) TestSecrets() {
	suite.RunCLI([]string{"gen", "secrets"}, base.StdoutEmpty())
	suite.Assert().FileExists("secrets.yaml")

	suite.RunCLI([]string{"gen", "secrets", "--output-file", "/tmp/secrets2.yaml"}, base.StdoutEmpty())
	suite.Assert().FileExists("/tmp/secrets2.yaml")

	suite.RunCLI([]string{"gen", "secrets", "-o", "secrets3.yaml", "--talos-version", "v0.8"}, base.StdoutEmpty())
	suite.Assert().FileExists("secrets3.yaml")
}

// TestConfigWithSecrets tests the gen config command with secrets provided.
func (suite *GenSuite) TestConfigWithSecrets() {
	suite.RunCLI([]string{"gen", "secrets"}, base.StdoutEmpty())
	suite.Assert().FileExists("secrets.yaml")

	secretsYaml, err := ioutil.ReadFile("secrets.yaml")
	suite.Assert().NoError(err)

	suite.RunCLI([]string{"gen", "config", "foo", "https://192.168.0.1:6443", "--with-secrets", "secrets.yaml"})

	config, err := configloader.NewFromFile("controlplane.yaml")
	suite.Assert().NoError(err)

	configSecretsBundle := generate.NewSecretsBundleFromConfig(generate.NewClock(), config)
	configSecretsBundleBytes, err := yaml.Marshal(configSecretsBundle)

	suite.Assert().NoError(err)
	suite.Assert().YAMLEq(string(secretsYaml), string(configSecretsBundleBytes))
}

func init() {
	allSuites = append(allSuites, new(GenSuite))
}
