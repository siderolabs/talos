// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_cli

package cli

import (
	"path/filepath"
	"regexp"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/siderolabs/talos/internal/integration/base"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
)

// TalosconfigSuite checks `talosctl config`.
type TalosconfigSuite struct {
	base.CLISuite
}

// SuiteName implements base.NamedSuite.
func (suite *TalosconfigSuite) SuiteName() string {
	return "cli.TalosconfigSuite"
}

// TestList checks `talosctl config contexts`.
func (suite *TalosconfigSuite) TestList() {
	suite.RunCLI([]string{"config", "contexts"},
		base.StdoutShouldMatch(regexp.MustCompile(`CURRENT`)))
}

// TestInfo checks `talosctl config info`.
func (suite *TalosconfigSuite) TestInfo() {
	suite.RunCLI([]string{"config", "info"}, // TODO: remove 10 years once the CABPT & TF providers are updated to 1.5.2+
		base.StdoutShouldMatch(regexp.MustCompile(`(1 year|10 years) from now`)))
}

// TestMerge checks `talosctl config merge`.
func (suite *TalosconfigSuite) TestMerge() {
	tempDir := suite.T().TempDir()

	suite.RunCLI([]string{"gen", "config", "-o", tempDir, "foo", "https://192.168.0.1:6443"},
		base.StdoutEmpty(),
		base.StderrNotEmpty(),
	)

	talosconfigPath := filepath.Join(tempDir, "talosconfig")

	suite.Assert().FileExists(talosconfigPath)

	path := filepath.Join(tempDir, "merged")

	suite.RunCLI([]string{"config", "merge", "--talosconfig", path, talosconfigPath},
		base.StdoutEmpty())

	suite.Require().FileExists(path)

	c, err := clientconfig.Open(path)
	suite.Require().NoError(err)

	suite.Require().NotNil(c.Contexts["foo"])

	suite.RunCLI([]string{"config", "merge", "--talosconfig", path, talosconfigPath},
		base.StdoutEmpty(),
		base.StderrNotEmpty(),
		base.StderrShouldMatch(regexp.MustCompile(`renamed`)),
	)

	c, err = clientconfig.Open(path)
	suite.Require().NoError(err)

	suite.Require().NotNil(c.Contexts["foo-1"])
}

// TestNew checks `talosctl config new`.
func (suite *TalosconfigSuite) TestNew() {
	stdout, _ := suite.RunCLI([]string{"version", "--json", "--nodes", suite.RandomDiscoveredNodeInternalIP()})

	var v machineapi.Version
	err := protojson.Unmarshal([]byte(stdout), &v)
	suite.Require().NoError(err)

	rbacEnabled := v.Features.GetRbac()

	tempDir := suite.T().TempDir()

	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeControlPlane)

	readerConfig := filepath.Join(tempDir, "readerconfig")
	suite.RunCLI([]string{"--nodes", node, "config", "new", "--roles", "os:reader", readerConfig},
		base.StdoutEmpty())

	operatorConfig := filepath.Join(tempDir, "operatorconfig")
	suite.RunCLI([]string{"--nodes", node, "config", "new", "--roles", "os:operator", operatorConfig},
		base.StdoutEmpty())

	// commands that work for admin, operator and reader, with and without RBAC
	for _, tt := range []struct {
		args []string
		opts []base.RunOption
	}{
		{
			args: []string{"ls", "/etc/hosts"},
			opts: []base.RunOption{base.StdoutShouldMatch(regexp.MustCompile(`hosts`))},
		},
	} {
		tt := tt
		name := strings.Join(tt.args, "_")
		suite.Run(name, func() {
			suite.T().Parallel()

			for _, config := range []string{readerConfig, operatorConfig} {
				args := append([]string{"--nodes", node}, tt.args...)
				suite.RunCLI(args, tt.opts...)

				args = append([]string{"--talosconfig", config}, args...)
				suite.RunCLI(args, tt.opts...)
			}
		})
	}

	// commands that work for admin, but not for reader&operator (when RBAC is enabled)
	for _, tt := range []struct {
		args        []string
		adminOpts   []base.RunOption
		nonprivOpts []base.RunOption
	}{
		{
			args:      []string{"read", "/etc/hosts"},
			adminOpts: []base.RunOption{base.StdoutShouldMatch(regexp.MustCompile(`localhost`))},
			nonprivOpts: []base.RunOption{
				base.StdoutEmpty(),
				base.StderrShouldMatch(regexp.MustCompile(`\Qrpc error: code = PermissionDenied desc = not authorized`)),
				base.ShouldFail(),
			},
		},
		{
			args:      []string{"get", "mc"},
			adminOpts: []base.RunOption{base.StdoutShouldMatch(regexp.MustCompile(`MachineConfig`))},
			nonprivOpts: []base.RunOption{
				base.ShouldFail(),
				base.StdoutShouldMatch(regexp.MustCompile(`\QNODE   NAMESPACE   TYPE   ID   VERSION`)),
				base.StderrShouldMatch(regexp.MustCompile(`\Qrpc error: code = PermissionDenied desc = not authorized`)),
			},
		},
		{
			args:      []string{"get", "osrootsecret"},
			adminOpts: []base.RunOption{base.StdoutShouldMatch(regexp.MustCompile(`OSRootSecret`))},
			nonprivOpts: []base.RunOption{
				base.ShouldFail(),
				base.StdoutShouldMatch(regexp.MustCompile(`\QNODE   NAMESPACE   TYPE   ID   VERSION`)),
				base.StderrShouldMatch(regexp.MustCompile(`\Qrpc error: code = PermissionDenied desc = not authorized`)),
			},
		},
		{
			args:      []string{"kubeconfig", "--force", tempDir},
			adminOpts: []base.RunOption{base.StdoutEmpty()},
			nonprivOpts: []base.RunOption{
				base.ShouldFail(),
				base.StdoutEmpty(),
				base.StderrShouldMatch(regexp.MustCompile(`\Qrpc error: code = PermissionDenied desc = not authorized`)),
			},
		},
	} {
		tt := tt
		name := strings.Join(tt.args, "_")
		suite.Run(name, func() {
			suite.T().Parallel()

			args := append([]string{"--nodes", node}, tt.args...)
			suite.RunCLI(args, tt.adminOpts...)

			for _, config := range []string{readerConfig, operatorConfig} {
				if rbacEnabled {
					suite.RunCLI(append([]string{"--talosconfig", config}, args...), tt.nonprivOpts...)
				} else {
					// check that it works the same way as for admin with reader's config
					suite.RunCLI(append([]string{"--talosconfig", config}, args...), tt.adminOpts...)
				}
			}
		})
	}

	// commands which work for operator, but not reader (when RBAC is enabled)
	for _, tt := range []struct {
		args        []string
		privOpts    []base.RunOption
		nonprivOpts []base.RunOption
	}{
		{
			args: []string{"etcd", "alarm", "list"},
			privOpts: []base.RunOption{
				base.StdoutEmpty(),
			},
			nonprivOpts: []base.RunOption{
				base.StdoutEmpty(),
				base.StderrShouldMatch(regexp.MustCompile(`\Qrpc error: code = PermissionDenied desc = not authorized`)),
				base.ShouldFail(),
			},
		},
	} {
		tt := tt
		name := strings.Join(tt.args, "_")
		suite.Run(name, func() {
			suite.T().Parallel()

			args := append([]string{"--nodes", node}, tt.args...)
			suite.RunCLI(args, tt.privOpts...)

			suite.RunCLI(append([]string{"--talosconfig", operatorConfig}, args...), tt.privOpts...)

			if rbacEnabled {
				suite.RunCLI(append([]string{"--talosconfig", readerConfig}, args...), tt.nonprivOpts...)
			} else {
				// check that it works the same way as for admin with reader's config
				suite.RunCLI(append([]string{"--talosconfig", readerConfig}, args...), tt.privOpts...)
			}
		})
	}

	// do not test destructive command with disabled RBAC
	if !rbacEnabled {
		return
	}

	// destructive commands that don't work for reader
	// (and that we don't test for admin because they are destructive)
	for _, tt := range []struct {
		args       []string
		readerOpts []base.RunOption
	}{
		{
			args: []string{"reboot", "--wait=false"},
			readerOpts: []base.RunOption{
				base.ShouldFail(),
				base.StdoutEmpty(),
				base.StderrShouldMatch(regexp.MustCompile(`\Qrpc error: code = PermissionDenied desc = not authorized`)),
			},
		},
		{
			args: []string{"reset", "--wait=false"},
			readerOpts: []base.RunOption{
				base.ShouldFail(),
				base.StdoutEmpty(),
				base.StderrShouldMatch(regexp.MustCompile(`\Qrpc error: code = PermissionDenied desc = not authorized`)),
			},
		},
	} {
		tt := tt
		name := strings.Join(tt.args, "_")
		suite.Run(name, func() {
			suite.T().Parallel()

			args := append([]string{"--nodes", node, "--talosconfig", readerConfig}, tt.args...)
			suite.RunCLI(args, tt.readerOpts...)
		})
	}
}

func init() {
	allSuites = append(allSuites, new(TalosconfigSuite))
}
