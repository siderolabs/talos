// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hardware_test

import (
	"slices"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hardware"
	runtimetalos "github.com/siderolabs/talos/internal/app/machined/pkg/runtime"
	"github.com/siderolabs/talos/internal/pkg/secureboot"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	hardwareres "github.com/siderolabs/talos/pkg/machinery/resources/hardware"
)

// pcrExtension is a single record of a PCR extension performed by the controller.
type pcrExtension struct {
	pcr  int
	data string
}

var (
	enterMachined = pcrExtension{pcr: constants.UKIPCR, data: string(secureboot.EnterMachined)}
	startTheWorld = pcrExtension{pcr: constants.UKIPCR, data: string(secureboot.StartTheWorld)}
)

// mockPCRExtender records the PCR extensions instead of talking to the TPM.
type mockPCRExtender struct {
	mu         sync.Mutex
	extensions []pcrExtension
}

func (m *mockPCRExtender) Extend(pcr int, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.extensions = append(m.extensions, pcrExtension{pcr: pcr, data: string(data)})

	return nil
}

func (m *mockPCRExtender) Extensions() []pcrExtension {
	m.mu.Lock()
	defer m.mu.Unlock()

	return slices.Clone(m.extensions)
}

type PCRStatusSuite struct {
	ctest.DefaultSuite

	extender *mockPCRExtender
}

func TestPCRStatusSuite(t *testing.T) {
	suite.Run(t, &PCRStatusSuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 30 * time.Second,
		},
	})
}

// startController registers the controller with a fresh mock extender.
func (suite *PCRStatusSuite) startController(mode runtimetalos.Mode) {
	suite.extender = &mockPCRExtender{}

	suite.Require().NoError(suite.Runtime().RegisterController(&hardware.PCRStatusController{
		V1Alpha1Mode: mode,
		TPMExtender:  suite.extender.Extend,
	}))
}

func (suite *PCRStatusSuite) createVolumeStatus(id string, volumeType block.VolumeType, phase block.VolumePhase) {
	volumeStatus := block.NewVolumeStatus(block.NamespaceName, id)
	volumeStatus.TypedSpec().Type = volumeType
	volumeStatus.TypedSpec().Phase = phase

	suite.Create(volumeStatus)
}

func (suite *PCRStatusSuite) updateVolumeStatusPhase(id string, phase block.VolumePhase) {
	volumeStatus, err := ctest.Get[*block.VolumeStatus](
		suite,
		block.NewVolumeStatus(block.NamespaceName, id).Metadata(),
	)
	suite.Require().NoError(err)

	volumeStatus.TypedSpec().Phase = phase

	suite.Update(volumeStatus)
}

func (suite *PCRStatusSuite) createVolumeConfig(id string, volumeType block.VolumeType) {
	volumeConfig := block.NewVolumeConfig(block.NamespaceName, id)
	volumeConfig.TypedSpec().Type = volumeType

	suite.Create(volumeConfig)
}

// assertExtended asserts that eventually exactly the expected extensions were performed.
func (suite *PCRStatusSuite) assertExtended(expected ...pcrExtension) {
	suite.T().Helper()

	// the assert helpers below run the condition in a separate goroutine which might outlive the test,
	// so capture the extender instead of accessing the suite field from it
	extender := suite.extender

	suite.Assert().EventuallyWithT(func(collect *assert.CollectT) {
		assert.Equal(collect, expected, extender.Extensions())
	}, 10*time.Second, 10*time.Millisecond)
}

// assertNotExtended asserts that the set of extensions performed stays as expected.
func (suite *PCRStatusSuite) assertNotExtended(expected ...pcrExtension) {
	suite.T().Helper()

	extender := suite.extender

	suite.Assert().Never(func() bool {
		return !slices.Equal(expected, extender.Extensions())
	}, time.Second, 20*time.Millisecond)
}

func (suite *PCRStatusSuite) assertPCRStatusExists() {
	suite.T().Helper()

	ctest.AssertResource(suite, strconv.Itoa(constants.UKIPCR), func(*hardwareres.PCRStatus, *assert.Assertions) {})
}

func (suite *PCRStatusSuite) assertPCRStatusDestroyed() {
	suite.T().Helper()

	ctest.AssertNoResource[*hardwareres.PCRStatus](suite, strconv.Itoa(constants.UKIPCR))
}

// bringUpRequiredVolumes creates STATE & EPHEMERAL volume statuses in the ready phase.
func (suite *PCRStatusSuite) bringUpRequiredVolumes() {
	suite.createVolumeStatus(constants.StatePartitionLabel, block.VolumeTypePartition, block.VolumePhaseReady)
	suite.createVolumeStatus(constants.EphemeralPartitionLabel, block.VolumeTypePartition, block.VolumePhaseReady)
}

// TestContainerMode verifies that the controller does nothing in a container.
func (suite *PCRStatusSuite) TestContainerMode() {
	suite.startController(runtimetalos.ModeContainer)

	suite.bringUpRequiredVolumes()

	suite.assertNotExtended()
	suite.assertPCRStatusDestroyed()
}

// TestInitialExtension verifies the initial PCR extension which unlocks disk encryption operations.
func (suite *PCRStatusSuite) TestInitialExtension() {
	suite.startController(runtimetalos.ModeMetal)

	suite.assertExtended(enterMachined)
	suite.assertPCRStatusExists()

	// no volumes are ready yet, so the PCR should not be locked
	suite.assertNotExtended(enterMachined)
}

// TestWaitForRequiredVolumes verifies that both STATE and EPHEMERAL should be ready before locking the PCR.
func (suite *PCRStatusSuite) TestWaitForRequiredVolumes() {
	suite.startController(runtimetalos.ModeMetal)

	suite.assertExtended(enterMachined)

	// only STATE is ready, EPHEMERAL is not even discovered yet
	suite.createVolumeStatus(constants.StatePartitionLabel, block.VolumeTypePartition, block.VolumePhaseReady)

	suite.assertNotExtended(enterMachined)
	suite.assertPCRStatusExists()

	// EPHEMERAL shows up, but is still being provisioned
	suite.createVolumeStatus(constants.EphemeralPartitionLabel, block.VolumeTypePartition, block.VolumePhaseProvisioned)

	suite.assertNotExtended(enterMachined)
	suite.assertPCRStatusExists()

	suite.updateVolumeStatusPhase(constants.EphemeralPartitionLabel, block.VolumePhaseReady)

	suite.assertExtended(enterMachined, startTheWorld)
	suite.assertPCRStatusDestroyed()
}

// TestWaitForPendingVolumeStatus verifies that any pending encryptable volume blocks the PCR extension.
func (suite *PCRStatusSuite) TestWaitForPendingVolumeStatus() {
	suite.startController(runtimetalos.ModeMetal)

	suite.assertExtended(enterMachined)

	suite.bringUpRequiredVolumes()
	suite.createVolumeStatus("USERDATA", block.VolumeTypePartition, block.VolumePhaseWaiting)

	suite.assertNotExtended(enterMachined)
	suite.assertPCRStatusExists()

	suite.updateVolumeStatusPhase("USERDATA", block.VolumePhaseReady)

	suite.assertExtended(enterMachined, startTheWorld)
	suite.assertPCRStatusDestroyed()
}

// TestWaitForPendingVolumeConfig verifies that a volume config without a matching status blocks the PCR extension.
func (suite *PCRStatusSuite) TestWaitForPendingVolumeConfig() {
	suite.startController(runtimetalos.ModeMetal)

	suite.assertExtended(enterMachined)

	suite.bringUpRequiredVolumes()
	suite.createVolumeConfig("USERDATA", block.VolumeTypeDisk)

	suite.assertNotExtended(enterMachined)
	suite.assertPCRStatusExists()

	// the volume status shows up, and the volume gets provisioned
	suite.createVolumeStatus("USERDATA", block.VolumeTypeDisk, block.VolumePhaseProvisioned)

	suite.assertNotExtended(enterMachined)

	suite.updateVolumeStatusPhase("USERDATA", block.VolumePhaseReady)

	suite.assertExtended(enterMachined, startTheWorld)
	suite.assertPCRStatusDestroyed()
}

// TestIgnoredVolumes verifies that volumes which can't be encrypted (and missing/closed ones) don't block the PCR extension.
func (suite *PCRStatusSuite) TestIgnoredVolumes() {
	suite.startController(runtimetalos.ModeMetal)

	suite.assertExtended(enterMachined)

	suite.bringUpRequiredVolumes()

	// not encryptable, so pending phases don't matter
	suite.createVolumeStatus("run", block.VolumeTypeTmpfs, block.VolumePhaseWaiting)
	suite.createVolumeStatus("/var/lib/foo", block.VolumeTypeDirectory, block.VolumePhaseFailed)
	suite.createVolumeConfig("etc-hosts", block.VolumeTypeSymlink)
	suite.createVolumeConfig("overlay-etc", block.VolumeTypeOverlay)

	// encryptable, but not going to be provisioned
	suite.createVolumeStatus("IMAGECACHE", block.VolumeTypePartition, block.VolumePhaseMissing)
	suite.createVolumeStatus("OLD", block.VolumeTypePartition, block.VolumePhaseClosed)

	suite.assertExtended(enterMachined, startTheWorld)
	suite.assertPCRStatusDestroyed()
}

// TestFinalizerBlocksExtension verifies that a locked PCRStatus (finalizer set) prevents the PCR extension.
func (suite *PCRStatusSuite) TestFinalizerBlocksExtension() {
	suite.startController(runtimetalos.ModeMetal)

	suite.assertExtended(enterMachined)
	suite.assertPCRStatusExists()

	pcrStatusMD := hardwareres.NewPCCRStatus(constants.UKIPCR).Metadata()

	suite.AddFinalizer(pcrStatusMD, "test")

	suite.bringUpRequiredVolumes()

	// the PCR is in use, so it can't be extended yet
	suite.assertNotExtended(enterMachined)
	ctest.AssertResource(suite, pcrStatusMD.ID(), func(r *hardwareres.PCRStatus, asrt *assert.Assertions) {
		asrt.Equal(resource.PhaseTearingDown, r.Metadata().Phase())
	})

	// removing the finalizer should wake up the controller on its own (destroy ready input)
	suite.RemoveFinalizer(pcrStatusMD, "test")

	suite.assertExtended(enterMachined, startTheWorld)
	suite.assertPCRStatusDestroyed()
}
