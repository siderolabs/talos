// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"io"
	"path/filepath"
	"time"

	"google.golang.org/grpc/codes"

	"github.com/siderolabs/talos/internal/integration/base"
	machineapi "github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// CGroupsSuite ...
type CGroupsSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *CGroupsSuite) SuiteName() string {
	return "api.CGroupsSuite"
}

// SetupTest ...
func (suite *CGroupsSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 5*time.Minute)
}

// TearDownTest ...
func (suite *CGroupsSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestCGroupsVersion tests that cgroups mount match expected version.
func (suite *CGroupsSuite) TestCGroupsVersion() {
	names := suite.listRootCgroup()

	suite.T().Log("detected cgroups v2")

	for _, subpath := range []string{
		"cgroup.controllers",
		"cgroup.max.depth",
		"cgroup.max.descendants",
		"cgroup.procs",
		"cgroup.stat",
		"cgroup.subtree_control",
		"cgroup.threads",
		"cpu.stat",
		"cpuset.cpus.effective",
		"cpuset.mems.effective",
		"init",
		"io.stat",
		"memory.numa_stat",
		"memory.stat",
		"podruntime",
		"system",
		constants.CgroupTalosContainersRoot,
		constants.CgroupVirtualMachines,
	} {
		suite.Assert().Contains(names, subpath)
	}
}

// TestCGroupsKubepods tests that kubelet creates the root cgroup for pods.
func (suite *CGroupsSuite) TestCGroupsKubepods() {
	if !suite.Capabilities().SupportsKubernetes {
		suite.T().Skip("cluster doesn't run Kubernetes, so kubelet never creates the kubepods cgroup")
	}

	suite.Assert().Contains(suite.listRootCgroup(), constants.CgroupKubepods)
}

// listRootCgroup returns the names of the entries in the root cgroup of a random node.
func (suite *CGroupsSuite) listRootCgroup() map[string]struct{} {
	node := suite.RandomDiscoveredNodeInternalIP()
	ctx := client.WithNode(suite.ctx, node)

	stream, err := suite.Client.MachineClient.List(ctx, &machineapi.ListRequest{Root: constants.CgroupMountPath})
	suite.Require().NoError(err)

	names := map[string]struct{}{}

	for {
		var info *machineapi.FileInfo

		info, err = stream.Recv()
		if err != nil {
			if err == io.EOF || client.StatusCode(err) == codes.Canceled {
				break
			}

			suite.Require().NoError(err)
		}

		names[filepath.Base(info.Name)] = struct{}{}
	}

	return names
}

func init() {
	allSuites = append(allSuites, new(CGroupsSuite))
}
