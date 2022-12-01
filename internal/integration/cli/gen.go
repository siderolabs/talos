// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_cli

package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1/generate"
)

// GenSuite verifies gen command.
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

	defer os.Remove("secrets.yaml") //nolint:errcheck

	suite.RunCLI([]string{"gen", "secrets", "--output-file", "/tmp/secrets2.yaml"}, base.StdoutEmpty())
	suite.Assert().FileExists("/tmp/secrets2.yaml")

	defer os.Remove("/tmp/secrets2.yaml") //nolint:errcheck

	suite.RunCLI([]string{"gen", "secrets", "-o", "secrets3.yaml", "--talos-version", "v0.8"}, base.StdoutEmpty())
	suite.Assert().FileExists("secrets3.yaml")

	defer os.Remove("secrets3.yaml") //nolint:errcheck
}

// TestSecretsWithPKIDirAndToken ...
func (suite *GenSuite) TestSecretsWithPKIDirAndToken() {
	path := "/tmp/secrets-with-pki-dir-and-token.yaml"

	tempDir := suite.T().TempDir()

	dir, err := writeKubernetesPKIFiles(tempDir)
	suite.Assert().NoError(err)

	defer os.RemoveAll(dir) //nolint:errcheck

	suite.RunCLI([]string{
		"gen", "secrets", "--from-kubernetes-pki", dir,
		"--kubernetes-bootstrap-token", "test-token",
		"--output-file", path,
	}, base.StdoutEmpty())

	suite.Assert().FileExists(path)

	defer os.Remove(path) //nolint:errcheck

	secretsYaml, err := os.ReadFile(path)
	suite.Assert().NoError(err)

	var secrets generate.SecretsBundle

	err = yaml.Unmarshal(secretsYaml, &secrets)
	suite.Assert().NoError(err)

	suite.Assert().Equal("test-token", secrets.Secrets.BootstrapToken, "bootstrap token does not match")
	suite.Assert().Equal(pkiCACrt, secrets.Certs.K8s.Crt, "k8s ca cert does not match")
	suite.Assert().Equal(pkiCAKey, secrets.Certs.K8s.Key, "k8s ca key does not match")
	suite.Assert().Equal(pkiFrontProxyCACrt, secrets.Certs.K8sAggregator.Crt, "k8s aggregator ca cert does not match")
	suite.Assert().Equal(pkiFrontProxyCAKey, secrets.Certs.K8sAggregator.Key, "k8s aggregator ca key does not match")
	suite.Assert().Equal(pkiSAKey, secrets.Certs.K8sServiceAccount.Key, "k8s service account key does not match")
	suite.Assert().Equal(pkiEtcdCACrt, secrets.Certs.Etcd.Crt, "etcd ca cert does not match")
	suite.Assert().Equal(pkiEtcdCAKey, secrets.Certs.Etcd.Key, "etcd ca key does not match")
}

// TestConfigWithSecrets tests the gen config command with secrets provided.
func (suite *GenSuite) TestConfigWithSecrets() {
	suite.RunCLI([]string{"gen", "secrets"}, base.StdoutEmpty())
	suite.Assert().FileExists("secrets.yaml")

	secretsYaml, err := os.ReadFile("secrets.yaml")
	suite.Assert().NoError(err)

	suite.RunCLI([]string{"gen", "config", "foo", "https://192.168.0.1:6443", "--with-secrets", "secrets.yaml"})

	config, err := configloader.NewFromFile("controlplane.yaml")
	suite.Assert().NoError(err)

	configSecretsBundle := generate.NewSecretsBundleFromConfig(generate.NewClock(), config)
	configSecretsBundleBytes, err := yaml.Marshal(configSecretsBundle)

	suite.Assert().NoError(err)
	suite.Assert().YAMLEq(string(secretsYaml), string(configSecretsBundleBytes))
}

// TestGenConfigWithDeprecatedOutputDirFlag tests that gen config command still works with the deprecated --output-dir flag.
func (suite *GenSuite) TestGenConfigWithDeprecatedOutputDirFlag() {
	tempDir := suite.T().TempDir()

	suite.RunCLI([]string{
		"gen", "config",
		"foo", "https://192.168.0.1:6443",
		"--output-dir", tempDir,
	})

	suite.Assert().FileExists(filepath.Join(tempDir, "controlplane.yaml"))
	suite.Assert().FileExists(filepath.Join(tempDir, "worker.yaml"))
	suite.Assert().FileExists(filepath.Join(tempDir, "talosconfig"))
}

// TestGenConfigToStdoutControlPlane tests that the gen config command can output a control plane config to stdout.
func (suite *GenSuite) TestGenConfigToStdoutControlPlane() {
	suite.RunCLI([]string{
		"gen", "config",
		"foo", "https://192.168.0.1:6443",
		"--output-types", "controlplane",
		"--output", "-",
	}, base.StdoutMatchFunc(func(output string) error {
		expected := "type: controlplane"
		if !strings.Contains(output, expected) {
			return fmt.Errorf("stdout does not contain %q: %q", expected, output)
		}

		return nil
	}))
}

// TestGenConfigToStdoutWorker tests that the gen config command can output a worker config to stdout.
func (suite *GenSuite) TestGenConfigToStdoutWorker() {
	suite.RunCLI([]string{
		"gen", "config",
		"foo", "https://192.168.0.1:6443",
		"--output-types", "worker",
		"--output", "-",
	}, base.StdoutMatchFunc(func(output string) error {
		expected := "type: worker"
		if !strings.Contains(output, expected) {
			return fmt.Errorf("stdout does not contain %q: %q", expected, output)
		}

		return nil
	}))
}

// TestGenConfigToStdoutTalosconfig tests that the gen config command can output a talosconfig to stdout.
func (suite *GenSuite) TestGenConfigToStdoutTalosconfig() {
	suite.RunCLI([]string{
		"gen", "config",
		"foo", "https://192.168.0.1:6443",
		"--output-types", "talosconfig",
		"--output", "-",
	}, base.StdoutMatchFunc(func(output string) error {
		expected := "context: foo"
		if !strings.Contains(output, expected) {
			return fmt.Errorf("stdout does not contain %q: %q", expected, output)
		}

		return nil
	}))
}

// TestGenConfigToStdoutMultipleTypesError tests that the gen config command fails when
// multiple output types are specified and output target is stdout.
func (suite *GenSuite) TestGenConfigToStdoutMultipleTypesError() {
	suite.RunCLI([]string{
		"gen", "config",
		"foo", "https://192.168.0.1:6443",
		"--output-types", "controlplane,worker",
		"--output", "-",
	}, base.StdoutEmpty(), base.ShouldFail(), base.StderrMatchFunc(func(output string) error {
		expected := "can't use multiple output types with stdout"
		if !strings.Contains(output, expected) {
			return fmt.Errorf("stderr does not contain %q: %q", expected, output)
		}

		return nil
	}))
}

// TestGenConfigMultipleTypesToDirectory tests that the gen config command works as expected
// when some output types are specified and output target is a directory.
func (suite *GenSuite) TestGenConfigMultipleTypesToDirectory() {
	tempDir := filepath.Join(suite.T().TempDir(), "inner")

	suite.RunCLI([]string{
		"gen", "config",
		"foo", "https://192.168.0.1:6443",
		"--output-types", "controlplane,worker",
		"--output", tempDir,
	})

	suite.Assert().FileExists(filepath.Join(tempDir, "controlplane.yaml"))
	suite.Assert().FileExists(filepath.Join(tempDir, "worker.yaml"))
	suite.Assert().NoFileExists(filepath.Join(tempDir, "talosconfig"))
}

// TestGenConfigSingleTypeToFile tests that the gen config command treats
// the output flag as a file path and not as a directory when a single output type is requested.
func (suite *GenSuite) TestGenConfigSingleTypeToFile() {
	tempFile := filepath.Join(suite.T().TempDir(), "worker-conf.yaml")

	suite.RunCLI([]string{
		"gen", "config",
		"foo", "https://192.168.0.1:6443",
		"--output-types", "worker",
		"--output", tempFile,
	})

	suite.Assert().FileExists(tempFile)
}

// TestGenConfigSingleTypeWithDeprecatedOutputDirFlagToDirectory tests that the gen config command treats
// the output flag still as a directory when the deprecated --output-dir flag is used.
func (suite *GenSuite) TestGenConfigSingleTypeWithDeprecatedOutputDirFlagToDirectory() {
	tempDir := filepath.Join(suite.T().TempDir(), "inner")

	suite.RunCLI([]string{
		"gen", "config",
		"foo", "https://192.168.0.1:6443",
		"--output-types", "worker",
		"--output-dir", tempDir,
	})

	suite.Assert().FileExists(filepath.Join(tempDir, "worker.yaml"))
}

// TestGenConfigInvalidOutputType tests that the gen config command fails when
// and invalid output type is requested.
func (suite *GenSuite) TestGenConfigInvalidOutputType() {
	suite.RunCLI([]string{
		"gen", "config",
		"foo", "https://192.168.0.1:6443",
		"--output-types", "worker,foobar",
	}, base.StdoutEmpty(), base.ShouldFail(), base.StderrMatchFunc(func(output string) error {
		expected := "invalid output type"
		if !strings.Contains(output, expected) {
			return fmt.Errorf("stderr does not contain %q: %q", expected, output)
		}

		return nil
	}))
}

// TestGenConfigNoOutputType tests that the gen config command fails when an empty list of output types is requested.
func (suite *GenSuite) TestGenConfigNoOutputType() {
	suite.RunCLI([]string{
		"gen", "config",
		"foo", "https://192.168.0.1:6443",
		"--output-types", "",
	}, base.StdoutEmpty(), base.ShouldFail(), base.StderrMatchFunc(func(output string) error {
		expected := "at least one output type must be specified"
		if !strings.Contains(output, expected) {
			return fmt.Errorf("stderr does not contain %q: %q", expected, output)
		}

		return nil
	}))
}

func init() {
	allSuites = append(allSuites, new(GenSuite))
}
