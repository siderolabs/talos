// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"bytes"
	"context"
	"io"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/protobuf/types/known/durationpb"

	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/client"
	clientconfig "github.com/siderolabs/talos/pkg/machinery/client/config"
	machinetype "github.com/siderolabs/talos/pkg/machinery/config/machine"
	hypervisorcfg "github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hardware"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/role"
)

// TestExplicitServiceKind preserves omitted-kind semantics on a live service.
func (suite *LogsSuite) TestExplicitServiceKind() {
	legacy, err := suite.Client.Logs(suite.nodeCtx, constants.SystemContainerdNamespace,
		common.ContainerDriver_CONTAINERD, "machined", false, 5)
	suite.Require().NoError(err)
	legacyBody, err := readKindLogs(legacy)
	suite.Require().NoError(err)
	suite.Require().NotEmpty(legacyBody)

	explicit, err := suite.Client.LogsWithKind(suite.nodeCtx, constants.SystemContainerdNamespace,
		common.ContainerDriver_CONTAINERD, "machined", false, 5, machine.LogKind_LOG_KIND_SERVICE)
	suite.Require().NoError(err)
	explicitBody, err := readKindLogs(explicit)
	suite.Require().NoError(err)
	// Do not compare a moving log tail byte-for-byte. Both routes must return
	// actual service data and honor the same line limit.
	suite.Require().NotEmpty(explicitBody)
	suite.Assert().LessOrEqual(bytes.Count(legacyBody, []byte("\n")), 5)
	suite.Assert().LessOrEqual(bytes.Count(explicitBody, []byte("\n")), 5)

	stream, err := suite.Client.LogsWithKind(suite.nodeCtx, constants.SystemContainerdNamespace,
		common.ContainerDriver_CONTAINERD, "machined", false, 5, machine.LogKind_LOG_KIND_VM)
	suite.Require().NoError(err)
	_, err = readKindLogs(stream)
	suite.Assert().Equal(missingVMLogCode(suite.T(), suite.nodeCtx, suite.Client), client.StatusCode(err),
		"VM selection must not fall back to service logs: %v", err)
}

// TestVMKindErrorsAndAuthorization uses node-issued constrained credentials.
func (suite *LogsSuite) TestVMKindErrorsAndAuthorization() {
	if testing.Short() {
		suite.T().Skip("skipping VM configuration changes in short mode")
	}

	node := suite.RandomDiscoveredNodeInternalIP()
	suite.nodeCtx = client.WithNode(suite.ctx, node)
	name := "vm-log-auth-" + uuid.NewString()
	unknown := "vm-log-unknown-" + uuid.NewString()
	original, err := suite.ReadConfigFromNode(suite.nodeCtx)
	suite.Require().NoError(err)
	originalBytes, err := original.Bytes()
	suite.Require().NoError(err)

	for _, doc := range original.VirtualMachineConfigs() {
		suite.Require().NotEqual(name, doc.Name())
		suite.Require().NotEqual(unknown, doc.Name())
	}

	rtestutils.AssertNoResource[*hypervisor.VirtualMachineSpec](suite.nodeCtx, suite.T(), suite.Client.COSI, name)
	rtestutils.AssertNoResource[*hypervisor.VirtualMachineSpec](suite.nodeCtx, suite.T(), suite.Client.COSI, unknown)
	suite.T().Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()

		cleanupCtx := client.WithNode(ctx, node)
		_, applyErr := suite.Client.ApplyConfiguration(cleanupCtx, &machine.ApplyConfigurationRequest{
			Data: originalBytes,
			Mode: machine.ApplyConfigurationRequest_NO_REBOOT,
		})
		suite.Assert().NoError(applyErr)
		rtestutils.AssertNoResource[*hypervisor.VirtualMachineSpec](cleanupCtx, suite.T(), suite.Client.COSI, name)
	})

	doc := hypervisorcfg.NewVirtualMachineConfigV1Alpha1()
	doc.MetaName = name
	doc.PowerStateConfig = hypervisorhelpers.PowerStateStopped
	doc.FirmwareConfig.FirmwareType = hypervisorhelpers.VirtualMachineFirmwareTypeBIOS
	doc.CPUConfig.CPUCount = 1
	doc.MemoryConfig.MemorySize = meta.MustByteSize("128MiB")
	doc.ConsoleConfig.SerialConfig.SerialEnabled = new(false)
	suite.PatchMachineConfig(suite.nodeCtx, doc)
	rtestutils.AssertResource(suite.nodeCtx, suite.T(), suite.Client.COSI, name,
		func(spec *hypervisor.VirtualMachineSpec, asrt *assert.Assertions) {
			asrt.False(spec.TypedSpec().Console.Serial)
		})
	// Serial configuration does not gate retained log access. Neither fixture
	// has a serial log; Docker nodes may also lack the required machine identity.
	missingCode := missingVMLogCode(suite.T(), suite.nodeCtx, suite.Client)
	suite.assertVMKindError(suite.Client, name, missingCode)
	suite.assertVMKindError(suite.Client, unknown, missingCode)

	doc.ConsoleConfig.SerialConfig.SerialEnabled = new(true)
	suite.PatchMachineConfig(suite.nodeCtx, doc)
	rtestutils.AssertResource(suite.nodeCtx, suite.T(), suite.Client.COSI, name,
		func(spec *hypervisor.VirtualMachineSpec, asrt *assert.Assertions) {
			asrt.True(spec.TypedSpec().Console.Serial)
		})

	version, err := suite.Client.Version(suite.nodeCtx)
	suite.Require().NoError(err)
	suite.Require().Len(version.Messages, 1)

	controlplanes := suite.DiscoverNodeInternalIPsByType(suite.ctx, machinetype.TypeControlPlane)

	for _, granted := range []role.Role{role.Reader, role.Operator} {
		suite.Run(string(granted), func() {
			if !version.Messages[0].Features.GetRbac() {
				suite.T().Skip("role enforcement requires RBAC enabled on the node")
			}

			if len(controlplanes) == 0 {
				suite.T().Skip("node-issued constrained credentials require a control-plane node")
			}

			issuanceCtx := client.WithNode(suite.ctx, controlplanes[0])
			response, generateErr := suite.Client.GenerateClientConfiguration(issuanceCtx, &machine.GenerateClientConfigurationRequest{
				Roles:  []string{string(granted)},
				CrtTtl: durationpb.New(time.Hour),
			})
			suite.Require().NoError(generateErr)
			suite.Require().Len(response.Messages, 1)
			cfg, configErr := clientconfig.FromBytes(response.Messages[0].Talosconfig)
			suite.Require().NoError(configErr)
			cfg.Contexts[cfg.Context].Endpoints = suite.Client.GetEndpoints()
			limited, clientErr := client.New(suite.ctx, client.WithConfig(cfg))

			suite.Require().NoError(clientErr)
			defer func() { suite.Assert().NoError(limited.Close()) }()

			// Positive control: these are working authenticated credentials,
			// and ordinary service logs remain accessible to these roles.
			stream, logsErr := limited.LogsWithKind(suite.nodeCtx, constants.SystemContainerdNamespace,
				common.ContainerDriver_CONTAINERD, "machined", false, 1, machine.LogKind_LOG_KIND_SERVICE)
			suite.Require().NoError(logsErr)

			body, readErr := readKindLogs(stream)
			suite.Require().NoError(readErr)
			suite.Require().NotEmpty(body)
			// VM logs use the same method-level authorization as service logs.
			suite.assertVMKindError(limited, name, missingCode)
			suite.assertVMKindError(limited, unknown, missingCode)
		})
	}
}

func (suite *LogsSuite) assertVMKindError(cli *client.Client, name string, code codes.Code) {
	suite.T().Helper()

	stream, err := cli.LogsWithKind(suite.nodeCtx, constants.SystemContainerdNamespace,
		common.ContainerDriver_CONTAINERD, name, false, -1, machine.LogKind_LOG_KIND_VM)
	if err == nil {
		_, err = readKindLogs(stream)
	}

	suite.Assert().Equal(code, client.StatusCode(err), "VM logs for %q returned %v", name, err)
}

// missingVMLogCode distinguishes a missing log from an unavailable path identity.
func missingVMLogCode(t *testing.T, ctx context.Context, cli *client.Client) codes.Code {
	t.Helper()

	system, err := safe.StateGetByID[*hardware.SystemInformation](ctx, cli.COSI, hardware.SystemInformationID)
	if state.IsNotFoundError(err) {
		return codes.FailedPrecondition
	}

	require.NoError(t, err)

	machineUUID, err := uuid.Parse(system.TypedSpec().UUID)
	if err != nil || machineUUID == uuid.Nil {
		return codes.FailedPrecondition
	}

	return codes.NotFound
}

func readKindLogs(stream machine.MachineService_LogsClient) ([]byte, error) {
	reader, err := client.ReadStream(stream)
	if err != nil {
		return nil, err
	}

	body, readErr := io.ReadAll(io.LimitReader(reader, (512<<10)+1))
	closeErr := reader.Close()

	if readErr != nil {
		return nil, readErr
	}

	if len(body) > 512<<10 {
		return nil, io.ErrShortBuffer
	}

	return body, closeErr
}
