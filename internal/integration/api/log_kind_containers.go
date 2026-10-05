// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"context"
	"fmt"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	hypervisorcfg "github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

// TestExplicitContainerKind distinguishes a container and configured VM with
// the same unique name without changing any real service configuration.
func (suite *ContainersSuite) TestExplicitContainerKind() {
	ctx, name, node := suite.setupContainer("log-kind")
	original, err := suite.ReadConfigFromNode(ctx)
	suite.Require().NoError(err)
	originalBytes, err := original.Bytes()
	suite.Require().NoError(err)
	rtestutils.AssertNoResource[*hypervisor.VirtualMachineSpec](ctx, suite.T(), suite.Client.COSI, name)
	suite.T().Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		cleanupCtx = client.WithNode(cleanupCtx, node)
		_, applyErr := suite.Client.ApplyConfiguration(cleanupCtx, &machine.ApplyConfigurationRequest{
			Data: originalBytes,
			Mode: machine.ApplyConfigurationRequest_NO_REBOOT,
		})
		suite.Assert().NoError(applyErr)
		rtestutils.AssertNoResource[*hypervisor.VirtualMachineSpec](cleanupCtx, suite.T(), suite.Client.COSI, name)
	})

	vm := hypervisorcfg.NewVirtualMachineConfigV1Alpha1()
	vm.MetaName = name
	vm.PowerStateConfig = hypervisorhelpers.PowerStateStopped
	vm.FirmwareConfig.FirmwareType = hypervisorhelpers.VirtualMachineFirmwareTypeBIOS
	vm.CPUConfig.CPUCount = 1
	vm.MemoryConfig.MemorySize = meta.MustByteSize("128MiB")
	vm.ConsoleConfig.SerialConfig.SerialEnabled = new(false)
	marker := "CONTAINER_KIND_" + uuid.NewString()
	suite.PatchMachineConfig(ctx, vm, suite.shellContainer(name, "printf '%s\\n' '"+marker+"'; sleep 3600"))
	suite.assertContainerLogged(ctx, name, marker)
	rtestutils.AssertResource(ctx, suite.T(), suite.Client.COSI, name,
		func(spec *hypervisor.VirtualMachineSpec, asrt *assert.Assertions) {
			asrt.False(spec.TypedSpec().Console.Serial)
		})

	for _, kind := range []machine.LogKind{machine.LogKind_LOG_KIND_UNSPECIFIED, machine.LogKind_LOG_KIND_CONTAINER} {
		stream, logsErr := suite.Client.LogsWithKind(ctx, constants.TalosContainersContainerdNamespace,
			common.ContainerDriver_CONTAINERD, name, false, -1, kind)
		suite.Require().NoError(logsErr)

		body, readErr := readKindLogs(stream)
		suite.Require().NoError(readErr)
		suite.Assert().Contains(string(body), marker)
	}

	// The same raw ID in system namespace selects a VM log file, not
	// the declared-container buffer. Serial-disabled configuration does not
	// prevent lookup, but this never-started VM has no log file.
	for _, test := range []struct {
		kind machine.LogKind
		code codes.Code
	}{
		{
			kind: machine.LogKind_LOG_KIND_VM,
			code: missingVMLogCode(suite.T(), ctx, suite.Client),
		},
		{
			kind: machine.LogKind_LOG_KIND_SERVICE,
			// Unregistered service buffers return a plain error, preserving
			// the legacy gRPC Unknown status rather than the VM NotFound status.
			code: codes.Unknown,
		},
	} {
		stream, logsErr := suite.Client.LogsWithKind(ctx, constants.SystemContainerdNamespace,
			common.ContainerDriver_CONTAINERD, name, false, -1, test.kind)
		if logsErr == nil {
			_, logsErr = readKindLogs(stream)
		}

		suite.Assert().Equal(test.code, client.StatusCode(logsErr), "kind %s returned %v", test.kind, logsErr)

		if test.kind == machine.LogKind_LOG_KIND_SERVICE {
			suite.Assert().Equal(fmt.Sprintf("log %q was not registered", name), status.Convert(logsErr).Message())
		}
	}
}
