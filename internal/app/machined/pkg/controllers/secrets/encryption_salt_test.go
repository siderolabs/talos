// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets_test

import (
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	secretsctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/secrets"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

type EncryptionSaltSuite struct {
	ctest.DefaultSuite
}

func (suite *EncryptionSaltSuite) TestDefault() {
	statePath := suite.T().TempDir()
	mountID := (&secretsctrl.EncryptionSaltController{}).Name() + "-" + constants.StatePartitionLabel

	ctest.AssertResource(suite, mountID, func(mountRequest *block.VolumeMountRequest, asrt *assert.Assertions) {
		asrt.Equal(constants.StatePartitionLabel, mountRequest.TypedSpec().VolumeID)
	})

	ctest.AssertNoResource[*secrets.EncryptionSalt](suite, secrets.EncryptionSaltID)

	volumeMountStatus := block.NewVolumeMountStatus(block.NamespaceName, mountID)
	volumeMountStatus.TypedSpec().Target = statePath
	suite.Create(volumeMountStatus)

	ctest.AssertResource(suite, secrets.EncryptionSaltID, func(*secrets.EncryptionSalt, *assert.Assertions) {})

	ctest.AssertResources(suite, []resource.ID{volumeMountStatus.Metadata().ID()}, func(vms *block.VolumeMountStatus, asrt *assert.Assertions) {
		asrt.True(vms.Metadata().Finalizers().Empty())
	})

	suite.Destroy(volumeMountStatus)

	ctest.AssertNoResource[*block.VolumeMountRequest](suite, mountID)

	suite.Assert().FileExists(filepath.Join(statePath, constants.EncryptionSaltFilename), "encryption salt file should exist")

	contents, err := os.ReadFile(filepath.Join(statePath, constants.EncryptionSaltFilename))
	suite.Require().NoError(err, "should be able to read encryption salt file")

	log.Printf("contents: %q", contents)
}

func (suite *EncryptionSaltSuite) TestLoad() {
	statePath := suite.T().TempDir()
	mountID := (&secretsctrl.EncryptionSaltController{}).Name() + "-" + constants.StatePartitionLabel

	ctest.AssertResource(suite, mountID, func(mountRequest *block.VolumeMountRequest, asrt *assert.Assertions) {
		asrt.Equal(constants.StatePartitionLabel, mountRequest.TypedSpec().VolumeID)
	})

	// using verbatim data here to make sure salt representation is supported in future version fo Talos
	suite.Require().NoError(os.WriteFile(filepath.Join(statePath, constants.EncryptionSaltFilename),
		[]byte("diskSalt:\n    - 240\n    - 180\n    - 79\n    - 128\n    - 31\n    - 0\n    - 19\n    - 124\n    - 165\n    - 74\n    - 113\n    - 220\n    - 27\n    - 83\n    - 46\n    - 74\n    - 204\n    - 190\n    - 217\n    - 96\n    - 221\n    - 2\n    - 165\n    - 98\n    - 245\n    - 36\n    - 165\n    - 151\n    - 149\n    - 66\n    - 113\n    - 16\n"), //nolint:lll
		0o600))

	ctest.AssertNoResource[*secrets.EncryptionSalt](suite, secrets.EncryptionSaltID)

	volumeMountStatus := block.NewVolumeMountStatus(block.NamespaceName, mountID)
	volumeMountStatus.TypedSpec().Target = statePath
	suite.Create(volumeMountStatus)

	ctest.AssertResource(suite, secrets.EncryptionSaltID, func(encryptionSalt *secrets.EncryptionSalt, asrt *assert.Assertions) {
		asrt.Equal(
			[]byte{0xf0, 0xb4, 0x4f, 0x80, 0x1f, 0x0, 0x13, 0x7c, 0xa5, 0x4a, 0x71, 0xdc, 0x1b, 0x53, 0x2e, 0x4a, 0xcc, 0xbe, 0xd9, 0x60, 0xdd, 0x2, 0xa5, 0x62, 0xf5, 0x24, 0xa5, 0x97, 0x95, 0x42, 0x71, 0x10},
			encryptionSalt.TypedSpec().DiskSalt,
		)
	})

	ctest.AssertResources(suite, []resource.ID{volumeMountStatus.Metadata().ID()}, func(vms *block.VolumeMountStatus, asrt *assert.Assertions) {
		asrt.True(vms.Metadata().Finalizers().Empty())
	})

	suite.Destroy(volumeMountStatus)

	ctest.AssertNoResource[*block.VolumeMountRequest](suite, mountID)
}

func TestEncryptionSaltSuite(t *testing.T) {
	t.Parallel()

	if os.Geteuid() != 0 {
		t.Skip("skipping test that requires root privileges")
	}

	suite.Run(t, &EncryptionSaltSuite{
		DefaultSuite: ctest.DefaultSuite{
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&secretsctrl.EncryptionSaltController{}))
			},
		},
	})
}
