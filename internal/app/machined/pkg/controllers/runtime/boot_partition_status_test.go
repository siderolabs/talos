// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	runtimecontrollers "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/runtime"
	machineruntime "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

type BootPartitionStatusSuite struct {
	ctest.DefaultSuite
}

func noEFI() (uuid.UUID, error) {
	return uuid.Nil, nil
}

func (suite *BootPartitionStatusSuite) createCmdline(cmdline string) {
	kernelCmdline := runtimeres.NewKernelCmdline()
	kernelCmdline.TypedSpec().Cmdline = cmdline
	suite.Create(kernelCmdline)
}

func (suite *BootPartitionStatusSuite) TestContainerMode() {
	suite.Require().NoError(suite.Runtime().RegisterController(&runtimecontrollers.BootPartitionStatusController{
		V1Alpha1Mode: machineruntime.ModeContainer,
		ReadEFIBootPartitionUUID: func() (uuid.UUID, error) {
			return uuid.MustParse("6f5a6e8a-9c79-4c0f-8f35-0c2a1f1a0001"), nil
		},
	}))

	suite.createCmdline("talos.platform=container talos.boot.partuuid=6f5a6e8a-9c79-4c0f-8f35-0c2a1f1a0002")

	// in container mode the devices are not ours, so the boot partition is never published
	ctest.AssertNoResource[*runtimeres.BootPartitionStatus](suite, runtimeres.BootPartitionStatusID)
}

func (suite *BootPartitionStatusSuite) TestEFI() {
	suite.Require().NoError(suite.Runtime().RegisterController(&runtimecontrollers.BootPartitionStatusController{
		V1Alpha1Mode: machineruntime.ModeMetal,
		ReadEFIBootPartitionUUID: func() (uuid.UUID, error) {
			return uuid.MustParse("6f5a6e8a-9c79-4c0f-8f35-0c2a1f1a0001"), nil
		},
	}))

	// no kernel cmdline yet => nothing to publish yet, as the kernel cmdline takes precedence
	ctest.AssertNoResource[*runtimeres.BootPartitionStatus](suite, runtimeres.BootPartitionStatusID)

	suite.createCmdline("talos.platform=metal console=ttyS0")

	ctest.AssertResource(suite, runtimeres.BootPartitionStatusID, func(r *runtimeres.BootPartitionStatus, asrt *assert.Assertions) {
		asrt.Equal("6f5a6e8a-9c79-4c0f-8f35-0c2a1f1a0001", r.TypedSpec().PartitionUUID)
	})
}

func (suite *BootPartitionStatusSuite) TestCmdlineOverridesEFI() {
	suite.Require().NoError(suite.Runtime().RegisterController(&runtimecontrollers.BootPartitionStatusController{
		V1Alpha1Mode: machineruntime.ModeMetal,
		ReadEFIBootPartitionUUID: func() (uuid.UUID, error) {
			return uuid.MustParse("6f5a6e8a-9c79-4c0f-8f35-0c2a1f1a0001"), nil
		},
	}))

	// on kexec, the EFI variable is stale (e.g. points to the ISO the machine was installed from),
	// while the kernel cmdline carries the boot partition of the kexec'ed kernel
	suite.createCmdline("talos.platform=metal talos.boot.partuuid=6f5a6e8a-9c79-4c0f-8f35-0c2a1f1a0002")

	ctest.AssertResource(suite, runtimeres.BootPartitionStatusID, func(r *runtimeres.BootPartitionStatus, asrt *assert.Assertions) {
		asrt.Equal("6f5a6e8a-9c79-4c0f-8f35-0c2a1f1a0002", r.TypedSpec().PartitionUUID)
	})
}

func (suite *BootPartitionStatusSuite) TestInvalidCmdlineFallbackToEFI() {
	suite.Require().NoError(suite.Runtime().RegisterController(&runtimecontrollers.BootPartitionStatusController{
		V1Alpha1Mode: machineruntime.ModeMetal,
		ReadEFIBootPartitionUUID: func() (uuid.UUID, error) {
			return uuid.MustParse("6f5a6e8a-9c79-4c0f-8f35-0c2a1f1a0001"), nil
		},
	}))

	suite.createCmdline("talos.platform=metal talos.boot.partuuid=not-a-uuid")

	ctest.AssertResource(suite, runtimeres.BootPartitionStatusID, func(r *runtimeres.BootPartitionStatus, asrt *assert.Assertions) {
		asrt.Equal("6f5a6e8a-9c79-4c0f-8f35-0c2a1f1a0001", r.TypedSpec().PartitionUUID)
	})
}

func (suite *BootPartitionStatusSuite) TestCmdline() {
	suite.Require().NoError(suite.Runtime().RegisterController(&runtimecontrollers.BootPartitionStatusController{
		V1Alpha1Mode:             machineruntime.ModeMetal,
		ReadEFIBootPartitionUUID: noEFI,
	}))

	// no kernel cmdline yet => nothing to publish yet
	ctest.AssertNoResource[*runtimeres.BootPartitionStatus](suite, runtimeres.BootPartitionStatusID)

	suite.createCmdline("talos.platform=metal talos.boot.partuuid=6F5A6E8A-9C79-4C0F-8F35-0C2A1F1A0002 console=ttyS0")

	ctest.AssertResource(suite, runtimeres.BootPartitionStatusID, func(r *runtimeres.BootPartitionStatus, asrt *assert.Assertions) {
		asrt.Equal("6f5a6e8a-9c79-4c0f-8f35-0c2a1f1a0002", r.TypedSpec().PartitionUUID)
	})
}

func (suite *BootPartitionStatusSuite) TestUnknown() {
	suite.Require().NoError(suite.Runtime().RegisterController(&runtimecontrollers.BootPartitionStatusController{
		V1Alpha1Mode:             machineruntime.ModeMetal,
		ReadEFIBootPartitionUUID: noEFI,
	}))

	// neither the EFI variable nor the kernel argument: the boot partition is not known
	suite.createCmdline("talos.platform=metal console=ttyS0")

	ctest.AssertNoResource[*runtimeres.BootPartitionStatus](suite, runtimeres.BootPartitionStatusID)
}

func (suite *BootPartitionStatusSuite) TestInvalidCmdline() {
	suite.Require().NoError(suite.Runtime().RegisterController(&runtimecontrollers.BootPartitionStatusController{
		V1Alpha1Mode:             machineruntime.ModeMetal,
		ReadEFIBootPartitionUUID: noEFI,
	}))

	suite.createCmdline("talos.platform=metal talos.boot.partuuid=not-a-uuid")

	ctest.AssertNoResource[*runtimeres.BootPartitionStatus](suite, runtimeres.BootPartitionStatusID)
}

func TestBootPartitionStatusSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &BootPartitionStatusSuite{
		Timeout: 5 * time.Second,
	})
}
