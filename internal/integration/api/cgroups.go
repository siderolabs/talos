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

	"github.com/siderolabs/go-procfs/procfs"
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
	node := suite.RandomDiscoveredNodeInternalIP()
	ctx := client.WithNode(suite.ctx, node)

	cmdline := suite.ReadCmdline(ctx)

	unified := procfs.NewCmdline(cmdline).Get(constants.KernelParamCGroups).First()
	cgroupsV1 := false

	if unified != nil && *unified == "0" {
		cgroupsV1 = true
	}

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

	if cgroupsV1 {
		suite.T().Log("detected cgroups v1")

		for _, subpath := range []string{
			"cpu",
			"cpuacct",
			"cpuset",
			"devices",
			"freezer",
			"memory",
			"net_cls",
			"net_prio",
			"perf_event",
			"pids",
		} {
			suite.Assert().Contains(names, subpath)
		}
	} else {
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
			"kubepods",
			"memory.numa_stat",
			"memory.stat",
			"podruntime",
			"system",
		} {
			suite.Assert().Contains(names, subpath)
		}
	}
}

func init() {
	allSuites = append(allSuites, new(CGroupsSuite))
}
