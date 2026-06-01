// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// ProcessSuite ...
type ProcessSuite struct {
	base.APISuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *ProcessSuite) SuiteName() string {
	return "api.ProcessSuite"
}

// SetupTest ...
func (suite *ProcessSuite) SetupTest() {
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 15*time.Second)

	if suite.Cluster == nil || suite.Cluster.Provisioner() != base.ProvisionerQEMU {
		// TODO: should we test caps and cgroups in Docker?
		suite.T().Skip("skipping process test since provisioner is not qemu")
	}
}

// TearDownTest ...
func (suite *ProcessSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

func (suite *ProcessSuite) readProcfs(nodeCtx context.Context, pid int32, property string) string {
	r, err := suite.Client.Read(nodeCtx, filepath.Join("/proc", strconv.Itoa(int(pid)), property))
	suite.Require().NoError(err)

	value, err := io.ReadAll(r)
	suite.Require().NoError(err)

	suite.Require().NoError(r.Close())

	return string(bytes.TrimSpace(value))
}

// TestProcessCapabilities reads capabilities of processes from procfs
// and validates system services get necessary capabilities dropped.
func (suite *ProcessSuite) TestProcessCapabilities() {
	nodes := suite.DiscoverNodeInternalIPs(suite.ctx)

	for _, node := range nodes {
		nodeCtx := client.WithNode(suite.ctx, node)

		r, err := suite.Client.Processes(nodeCtx)
		suite.Require().NoError(err)

		found := 0

		for _, msg := range r.Messages {
			procs := msg.Processes

			for _, p := range procs {
				switch p.Command {
				case "systemd-udevd":
					found++

					// All but cap_sys_boot
					suite.Require().Contains(
						suite.readProcfs(nodeCtx, p.Pid, "status"),
						"CapPrm:\t000001ffffbfffff\nCapEff:\t000001ffffbfffff\nCapBnd:\t000001ffffbfffff",
					)

					suite.Require().Equal(
						suite.readProcfs(nodeCtx, p.Pid, "cgroup"),
						"0::/system/udevd",
					)

					suite.Require().Contains(
						suite.readProcfs(nodeCtx, p.Pid, "environ"),
						constants.EnvXDGRuntimeDir,
					)
					suite.Require().Contains(
						suite.readProcfs(nodeCtx, p.Pid, "status"),
						"Uid:\t0",
					)
				case "dashboard":
					found++

					// None
					suite.Require().Contains(
						suite.readProcfs(nodeCtx, p.Pid, "status"),
						"CapPrm:\t0000000000000000\nCapEff:\t0000000000000000\nCapBnd:\t0000000000000000",
					)

					suite.Require().Equal(
						suite.readProcfs(nodeCtx, p.Pid, "cgroup"),
						"0::/system/dashboard",
					)

					suite.Require().Equal(
						suite.readProcfs(nodeCtx, p.Pid, "oom_score_adj"),
						"-400",
					)
					suite.Require().Contains(
						suite.readProcfs(nodeCtx, p.Pid, "environ"),
						"TERM=linux",
					)
					suite.Require().Contains(
						suite.readProcfs(nodeCtx, p.Pid, "status"),
						"Uid:\t50",
					)
				case "containerd":
					found++

					// All but cap_sys_boot, cap_sys_module
					suite.Require().Contains(
						suite.readProcfs(nodeCtx, p.Pid, "status"),
						"CapPrm:\t000001ffffbeffff\nCapEff:\t000001ffffbeffff\nCapBnd:\t000001ffffbeffff",
					)

					if strings.Contains(p.Args, "/system/run/containerd") {
						suite.Require().Equal(
							suite.readProcfs(nodeCtx, p.Pid, "cgroup"),
							"0::/system/runtime",
						)

						suite.Require().Equal(
							suite.readProcfs(nodeCtx, p.Pid, "oom_score_adj"),
							"-999",
						)
					} else {
						suite.Require().Equal(
							suite.readProcfs(nodeCtx, p.Pid, "cgroup"),
							"0::/podruntime/runtime",
						)

						suite.Require().Equal(
							suite.readProcfs(nodeCtx, p.Pid, "oom_score_adj"),
							"-500",
						)
					}

					suite.Require().Contains(
						suite.readProcfs(nodeCtx, p.Pid, "environ"),
						constants.EnvXDGRuntimeDir,
					)
					suite.Require().Contains(
						suite.readProcfs(nodeCtx, p.Pid, "status"),
						"Uid:\t0",
					)
				}
			}
		}

		suite.Require().Equal(4, found, "Not all processes found")
	}
}

func init() {
	allSuites = append(allSuites, new(ProcessSuite))
}
