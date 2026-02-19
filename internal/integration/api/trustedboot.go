// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration_api

package api

import (
	"bufio"
	"bytes"
	"context"
	"io"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/integration/base"
	"github.com/siderolabs/talos/pkg/machinery/client"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	blockcfg "github.com/siderolabs/talos/pkg/machinery/config/types/block"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// TrustedBootSuite verifies Talos is securebooted.
type TrustedBootSuite struct {
	base.K8sSuite

	ctx       context.Context //nolint:containedctx
	ctxCancel context.CancelFunc
}

// SuiteName ...
func (suite *TrustedBootSuite) SuiteName() string {
	return "api.TrustedBootSuite"
}

// SetupTest ...
func (suite *TrustedBootSuite) SetupTest() {
	if !suite.TrustedBoot {
		suite.T().Skip("skipping since talos.trustedboot is false")
	}

	// make sure API calls have timeout
	suite.ctx, suite.ctxCancel = context.WithTimeout(context.Background(), 10*time.Minute)
}

// TearDownTest ...
func (suite *TrustedBootSuite) TearDownTest() {
	if suite.ctxCancel != nil {
		suite.ctxCancel()
	}
}

// TestTrustedBootState verifies that the system is booted in secure boot mode
// and that the disks are encrypted.
func (suite *TrustedBootSuite) TestTrustedBootState() {
	node := suite.RandomDiscoveredNodeInternalIP()
	ctx := client.WithNode(suite.ctx, node)

	rtestutils.AssertResources(ctx, suite.T(), suite.Client.COSI, []resource.ID{runtimeres.SecurityStateID},
		func(r *runtimeres.SecurityState, asrt *assert.Assertions) {
			asrt.True(r.TypedSpec().SecureBoot)
		},
	)

	rtestutils.AssertResources(ctx, suite.T(), suite.Client.COSI,
		[]resource.ID{constants.StatePartitionLabel, constants.EphemeralPartitionLabel},
		func(r *block.VolumeStatus, asrt *assert.Assertions) {
			asrt.Equal(block.VolumePhaseReady, r.TypedSpec().Phase)
			asrt.Equal(block.EncryptionProviderLUKS2, r.TypedSpec().EncryptionProvider)
		},
	)

	dmesgStream, err := suite.Client.Dmesg(
		suite.ctx,
		false,
		false,
	)
	suite.Require().NoError(err)

	logReader, err := client.ReadStream(dmesgStream)
	suite.Require().NoError(err)

	var dmesg bytes.Buffer

	_, err = io.Copy(bufio.NewWriter(&dmesg), logReader)
	suite.Require().NoError(err)

	suite.Require().Contains(dmesg.String(), "Secure boot enabled")
}

// TestEncryptionConfigRotate verifies that the encryption supports locking and unlocking to different PCRs.
func (suite *TrustedBootSuite) TestEncryptionConfigRotate() {
	node := suite.RandomDiscoveredNodeInternalIP(machine.TypeWorker)
	suite.ClearConnectionRefused(suite.ctx, node)

	nodeCtx := client.WithNode(suite.ctx, node)

	provider, err := suite.ReadConfigFromNode(nodeCtx)
	suite.Require().NoError(err)

	ephemeralCfg, _ := provider.Volumes().ByName(constants.EphemeralPartitionLabel)
	encryption := ephemeralCfg.Encryption()

	suite.Require().NotNil(encryption, "encryption config must be set for EPHEMERAL in trustedboot test")

	suite.WaitForBootDone(suite.ctx)

	suite.T().Logf("testing encryption key rotation on node %s", node)

	cfg, ok := encryption.(blockcfg.EncryptionSpec)
	suite.Require().True(ok, "expected blockcfg.EncryptionSpec, got %T", encryption)

	// when we start the test we do not know the current encryption provider in use
	// so let's read the volumestatus to get the information about the slot in use and whether lockToState is set
	volumeStatus, err := safe.StateGetByID[*block.VolumeStatus](nodeCtx, suite.Client.COSI, constants.EphemeralPartitionLabel)
	suite.Require().NoError(err)

	existingEncryptionSlotInUse := *volumeStatus.TypedSpec().EncryptionSlot

	existing := xslices.Filter(cfg.EncryptionKeys, func(key blockcfg.EncryptionKey) bool {
		return key.Slot() == existingEncryptionSlotInUse
	})[0]

	var (
		existingPCRs       []int
		expectedPubKeyPCRs []int
	)

	if existing.TPM() != nil {
		existingPCRs = existing.TPM().PCRs()
		expectedPubKeyPCRs = existing.TPM().PubKeyPCRs()
	}

	nextSlot := existing.Slot() + 1

	for _, test := range []struct {
		keys []blockcfg.EncryptionKey

		expectedPCRs          []int
		expectedPubKeyPCRs    []int
		expectedLockedToState bool
	}{
		// for the initial set, let's add a new TPM based key with no PCR options specified
		// in this case after reboot the new slot will be added for the TPM key and the expected PCRs
		// and lockToState status will be the same as the existing key
		{
			keys: []blockcfg.EncryptionKey{
				existing,
				{
					KeyTPM: &blockcfg.EncryptionKeyTPM{
						// TPMCheckSecurebootStatusOnEnroll: new(true),
					},
					KeySlot:        nextSlot,
					KeyLockToSTATE: new(true),
				},
			},

			expectedPCRs:          existingPCRs,
			expectedPubKeyPCRs:    expectedPubKeyPCRs,
			expectedLockedToState: existing.LockToSTATE(),
		},
		// now remove the existing key and add a new TPM based key with no PCRs specified
		// in this case after a reboot we should have default TPM based encryption values
		// i.e. PCR is SecureBootStatePCR and lockToState is true
		{
			keys: []blockcfg.EncryptionKey{
				{
					KeyTPM: &blockcfg.EncryptionKeyTPM{
						// TPMCheckSecurebootStatusOnEnroll: new(true),
					},
					KeySlot:        nextSlot,
					KeyLockToSTATE: new(true),
				},
			},

			expectedPCRs:          []int{constants.SecureBootStatePCR},
			expectedPubKeyPCRs:    []int{constants.UKIPCR},
			expectedLockedToState: true,
		},
		// now keep the previous TPM based key with no PCRs specified and add a new key with PCRs set
		// to empty and lockToState set to false, after reboot we should have default TPM based encryption values
		// i.e. PCR is SecureBootStatePCR and lockToState is true
		{
			keys: []blockcfg.EncryptionKey{
				{
					KeyTPM: &blockcfg.EncryptionKeyTPM{
						// TPMCheckSecurebootStatusOnEnroll: new(true),
					},
					KeySlot:        nextSlot,
					KeyLockToSTATE: new(true),
				},
				{
					KeyTPM: &blockcfg.EncryptionKeyTPM{
						// TPMCheckSecurebootStatusOnEnroll: new(true),
						TPMOptions: &blockcfg.EncryptionKeyTPMOptions{
							PCRs: []int{},
						},
					},
					KeySlot:        nextSlot + 1,
					KeyLockToSTATE: new(false),
				},
			},

			expectedPCRs:          []int{constants.SecureBootStatePCR},
			expectedPubKeyPCRs:    []int{constants.UKIPCR},
			expectedLockedToState: true,
		},
		// now only keep the TPM key with PCRs set to empty and lockToState set to false
		// in this case after a reboot we should have no PCRs and lockToState is false
		{
			keys: []blockcfg.EncryptionKey{
				{
					KeyTPM: &blockcfg.EncryptionKeyTPM{
						// TPMCheckSecurebootStatusOnEnroll: new(true),
						TPMOptions: &blockcfg.EncryptionKeyTPMOptions{
							PCRs: []int{},
						},
					},
					KeySlot:        nextSlot + 1,
					KeyLockToSTATE: new(false),
				},
			},

			expectedPCRs:          nil,
			expectedPubKeyPCRs:    []int{constants.UKIPCR},
			expectedLockedToState: false,
		},
		// now keep the previous TPM based key with PCRs set to empty and lockToState set to false
		// and add a new key with PCRs set to [0, SecureBootStatePCR] and lockToState set to true
		// in this case after a reboot we should have no PCRs and lockToState is false
		{
			keys: []blockcfg.EncryptionKey{
				{
					KeyTPM: &blockcfg.EncryptionKeyTPM{
						// TPMCheckSecurebootStatusOnEnroll: new(true),
						TPMOptions: &blockcfg.EncryptionKeyTPMOptions{
							PCRs: []int{},
						},
					},
					KeySlot:        nextSlot + 1,
					KeyLockToSTATE: new(false),
				},
				{
					KeyTPM: &blockcfg.EncryptionKeyTPM{
						// TPMCheckSecurebootStatusOnEnroll: new(true),
						TPMOptions: &blockcfg.EncryptionKeyTPMOptions{
							PCRs: []int{0, constants.SecureBootStatePCR},
						},
					},
					KeySlot:        nextSlot + 2,
					KeyLockToSTATE: new(true),
				},
			},

			expectedPCRs:          nil,
			expectedPubKeyPCRs:    []int{constants.UKIPCR},
			expectedLockedToState: false,
		},
		// now only keep the TPM key with PCRs set to [0, SecureBootStatePCR] and lockToState set to true
		// in this case after a reboot we should have PCRs set to [0, SecureBootStatePCR] and lockToState is true
		{
			keys: []blockcfg.EncryptionKey{
				{
					KeyTPM: &blockcfg.EncryptionKeyTPM{
						// TPMCheckSecurebootStatusOnEnroll: new(true),
						TPMOptions: &blockcfg.EncryptionKeyTPMOptions{
							PCRs: []int{0, constants.SecureBootStatePCR},
						},
					},
					KeySlot:        nextSlot + 2,
					KeyLockToSTATE: new(true),
				},
			},

			expectedPCRs:          []int{0, constants.SecureBootStatePCR},
			expectedPubKeyPCRs:    []int{constants.UKIPCR},
			expectedLockedToState: true,
		},
	} {
		suite.T().Logf("applying encryption keys %s on node %s", toJSONString(suite.T(), test.keys), node)

		// prepare a patch to apply, first removing existing keys
		removeKeysPatch := map[string]any{
			"apiVersion": "v1alpha1",
			"kind":       "VolumeConfig",
			"name":       constants.EphemeralPartitionLabel,
			"encryption": map[string]any{
				"keys": map[string]any{
					"$patch": "delete",
				},
			},
		}

		newEphemeralCfg := blockcfg.NewVolumeConfigV1Alpha1()
		newEphemeralCfg.MetaName = constants.EphemeralPartitionLabel
		newEphemeralCfg.EncryptionSpec.EncryptionKeys = test.keys

		// right now, patching encryption keys doesn't reboot and doesn't rotate the secrets either
		suite.PatchMachineConfig(nodeCtx, removeKeysPatch, newEphemeralCfg)

		suite.AssertRebooted(
			suite.ctx, node,
			func(nodeCtx context.Context) error {
				return base.IgnoreGRPCUnavailable(suite.Client.Reboot(nodeCtx))
			}, assertRebootedRebootTimeout,
			suite.CleanupFailedPods,
		)

		suite.ClearConnectionRefused(suite.ctx, node)

		suite.WaitForBootDone(suite.ctx)

		rtestutils.AssertResource(nodeCtx, suite.T(), suite.Client.COSI, constants.EphemeralPartitionLabel, func(r *block.VolumeStatus, asrt *assert.Assertions) {
			asrt.Equal(test.expectedPCRs, r.TypedSpec().TPMEncryptionOptions.PCRs)
			asrt.Equal(test.expectedPubKeyPCRs, r.TypedSpec().TPMEncryptionOptions.PubKeyPCRs)
			asrt.Equal(test.expectedLockedToState, r.TypedSpec().EncryptionLockedToState)
		})
	}
}

func init() {
	allSuites = append(allSuites, &TrustedBootSuite{})
}
