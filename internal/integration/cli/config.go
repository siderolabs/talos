// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_cli
// +build integration_cli

package cli

import (
	"path/filepath"
	"regexp"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"

	"github.com/talos-systems/talos/internal/integration/base"
	machineapi "github.com/talos-systems/talos/pkg/machinery/api/machine"
	clientconfig "github.com/talos-systems/talos/pkg/machinery/client/config"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1/machine"
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

// TestMerge checks `talosctl config merge`.
func (suite *TalosconfigSuite) TestMerge() {
	tempDir := suite.T().TempDir()

	suite.RunCLI([]string{"gen", "config", "-o", tempDir, "foo", "https://192.168.0.1:6443"})

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
		base.StdoutShouldMatch(regexp.MustCompile(`renamed`)))

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

	readerConfig := filepath.Join(tempDir, "talosconfig")
	suite.RunCLI([]string{"--nodes", node, "config", "new", "--roles", "os:reader", readerConfig},
		base.StdoutEmpty())

	// commands that work for both admin and reader, with and without RBAC
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

			args := append([]string{"--nodes", node}, tt.args...)
			suite.RunCLI(args, tt.opts...)

			args = append([]string{"--talosconfig", readerConfig}, args...)
			suite.RunCLI(args, tt.opts...)
		})
	}

	// commands that work for admin, but not for reader (when RBAC is enabled)
	for _, tt := range []struct {
		args       []string
		adminOpts  []base.RunOption
		readerOpts []base.RunOption
	}{
		{
			args:      []string{"read", "/etc/hosts"},
			adminOpts: []base.RunOption{base.StdoutShouldMatch(regexp.MustCompile(`localhost`))},
			readerOpts: []base.RunOption{
				base.StdoutEmpty(),
				base.StderrShouldMatch(regexp.MustCompile(`\Qrpc error: code = PermissionDenied desc = not authorized`)),
				base.ShouldFail(),
			},
		},
		{
			args:      []string{"get", "mc"},
			adminOpts: []base.RunOption{base.StdoutShouldMatch(regexp.MustCompile(`MachineConfig`))},
			readerOpts: []base.RunOption{
				base.StdoutEmpty(),
				base.StderrShouldMatch(regexp.MustCompile(`\Qrpc error: code = PermissionDenied desc = not authorized`)),
			},
		},
		{
			args:      []string{"get", "osrootsecret"},
			adminOpts: []base.RunOption{base.StdoutShouldMatch(regexp.MustCompile(`OSRootSecret`))},
			readerOpts: []base.RunOption{
				base.StdoutEmpty(),
				base.StderrShouldMatch(regexp.MustCompile(`\Qrpc error: code = PermissionDenied desc = not authorized`)),
			},
		},
		{
			args:      []string{"kubeconfig", "--force", tempDir},
			adminOpts: []base.RunOption{base.StdoutEmpty()},
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

			args := append([]string{"--nodes", node}, tt.args...)
			suite.RunCLI(args, tt.adminOpts...)

			args = append([]string{"--talosconfig", readerConfig}, args...)
			if rbacEnabled {
				suite.RunCLI(args, tt.readerOpts...)
			} else {
				// check that it works the same way as for admin with reader's config
				suite.RunCLI(args, tt.adminOpts...)
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
			args: []string{"reboot"},
			readerOpts: []base.RunOption{
				base.ShouldFail(),
				base.StdoutEmpty(),
				base.StderrShouldMatch(regexp.MustCompile(`\Qrpc error: code = PermissionDenied desc = not authorized`)),
			},
		},
		{
			args: []string{"reset"},
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
