// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	hypervisorctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor"
	configcfg "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	blockcfg "github.com/siderolabs/talos/pkg/machinery/config/types/block"
	hypervisorcfg "github.com/siderolabs/talos/pkg/machinery/config/types/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

const (
	// testLibrary is the content library name the tests use.
	testLibrary = "vm-images"
	// testVolumeName is the name the backing volume is declared under, as written by the user.
	testVolumeName = "vm-images"
	// contentLibraryControllerName mirrors the controller's name, which its mount requests are keyed by.
	contentLibraryControllerName = "hypervisor.ContentLibraryController"
)

// testVolumeID is the internal ID the backing volume gets, which the user never writes.
var testVolumeID = constants.UserVolumePrefix + testVolumeName

// testRequestID mirrors the controller's naming so tests can find what it creates.
var testRequestID = contentLibraryControllerName + "/" + testLibrary + "/" + testVolumeID

type ContentLibrarySuite struct {
	ctest.DefaultSuite
}

func TestContentLibrarySuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &ContentLibrarySuite{
		DefaultSuite: ctest.DefaultSuite{
			Timeout: 15 * time.Second,
			AfterSetup: func(suite *ctest.DefaultSuite) {
				suite.Require().NoError(suite.Runtime().RegisterController(&hypervisorctrl.ContentLibraryController{}))
			},
		},
	})
}

// applyConfig puts the given documents into the active machine config.
func (suite *ContentLibrarySuite) applyConfig(docs ...configcfg.Document) {
	cfg, err := container.New(docs...)
	suite.Require().NoError(err)

	suite.Require().NoError(suite.State().Create(suite.Ctx(), config.NewMachineConfig(cfg)))
}

// updateConfig replaces the active machine config with the given documents.
func (suite *ContentLibrarySuite) updateConfig(docs ...configcfg.Document) {
	cfg, err := container.New(docs...)
	suite.Require().NoError(err)

	old, err := safe.StateGetByID[*config.MachineConfig](suite.Ctx(), suite.State(), config.ActiveID)
	suite.Require().NoError(err)

	res := config.NewMachineConfig(cfg)
	res.Metadata().SetVersion(old.Metadata().Version())

	suite.Require().NoError(suite.State().Update(suite.Ctx(), res))
}

// applyLibrary puts the library into the machine config, along with the user volume backing it.
//
// Resolution needs the volume document: the library names it, and the document declaring that name
// is what decides the volume's ID.
func (suite *ContentLibrarySuite) applyLibrary(doc *hypervisorcfg.ContentLibraryConfigV1Alpha1) {
	suite.applyConfig(newUserVolume(testVolumeName), doc)
}

func newDoc() *hypervisorcfg.ContentLibraryConfigV1Alpha1 {
	return newDocBackedBy(testVolumeName)
}

// newDocBackedBy builds the test library, backed by the named volume.
func newDocBackedBy(volumeName string) *hypervisorcfg.ContentLibraryConfigV1Alpha1 {
	return newNamedDoc(testLibrary, volumeName)
}

// newNamedDoc builds a library under the given name, backed by the named volume.
func newNamedDoc(libraryID, volumeName string) *hypervisorcfg.ContentLibraryConfigV1Alpha1 {
	doc := hypervisorcfg.NewContentLibraryConfigV1Alpha1()
	doc.MetaName = libraryID
	doc.BackingConfig.VolumeName = volumeName

	return doc
}

func newUserVolume(name string) *blockcfg.UserVolumeConfigV1Alpha1 {
	doc := blockcfg.NewUserVolumeConfigV1Alpha1()
	doc.MetaName = name

	return doc
}

func newExternalVolume(name string) *blockcfg.ExternalVolumeConfigV1Alpha1 {
	doc := blockcfg.NewExternalVolumeConfigV1Alpha1()
	doc.MetaName = name

	return doc
}

func newExistingVolume(name string) *blockcfg.ExistingVolumeConfigV1Alpha1 {
	doc := blockcfg.NewExistingVolumeConfigV1Alpha1()
	doc.MetaName = name

	return doc
}

// newReadOnlyExternalVolume builds an external volume the user declared read-only.
func newReadOnlyExternalVolume(name string) *blockcfg.ExternalVolumeConfigV1Alpha1 {
	doc := newExternalVolume(name)
	doc.MountSpec.MountReadOnly = new(true)

	return doc
}

// newReadOnlyExistingVolume builds an existing volume the user declared read-only.
func newReadOnlyExistingVolume(name string) *blockcfg.ExistingVolumeConfigV1Alpha1 {
	doc := newExistingVolume(name)
	doc.MountSpec.MountReadOnly = new(true)

	return doc
}

// satisfyMount creates the VolumeMountStatus the block subsystem would produce for the request.
func (suite *ContentLibrarySuite) satisfyMount(requestID, volumeID, target string, readOnly bool) {
	status := block.NewVolumeMountStatus(block.NamespaceName, requestID)
	status.TypedSpec().VolumeID = volumeID
	status.TypedSpec().Requester = contentLibraryControllerName
	status.TypedSpec().Target = target
	status.TypedSpec().ReadOnly = readOnly

	suite.Require().NoError(suite.State().Create(suite.Ctx(), status))
}

func (suite *ContentLibrarySuite) assertNotReady(expectedError string) {
	ctest.AssertResource(suite, testLibrary, func(status *hypervisor.ContentLibraryStatus, asrt *assert.Assertions) {
		asrt.False(status.TypedSpec().Ready)
		asrt.Equal(expectedError, status.TypedSpec().Error)
		asrt.Empty(status.TypedSpec().Path)
	})
}

func (suite *ContentLibrarySuite) assertHeld(requestID string, held bool) {
	ctest.AssertResource(suite, requestID, func(status *block.VolumeMountStatus, asrt *assert.Assertions) {
		asrt.Equal(held, status.Metadata().Finalizers().Has(contentLibraryControllerName))
	})
}

// TestResolvesUserVolume covers a library backed by a user volume: the name resolves to the volume's
// internal ID, which is what the status reports.
func (suite *ContentLibrarySuite) TestResolvesUserVolume() {
	suite.applyLibrary(newDoc())

	ctest.AssertResource(suite, testLibrary, func(status *hypervisor.ContentLibraryStatus, asrt *assert.Assertions) {
		asrt.Equal(testVolumeID, status.TypedSpec().VolumeID)
	})

	// The mount is asked for, and nothing has answered yet.
	suite.assertNotReady("waiting for the backing volume to be mounted")
}

// TestResolvesExternalVolume covers a library backed by an external volume: the same name, but the
// document declaring it gives the volume a different ID.
func (suite *ContentLibrarySuite) TestResolvesExternalVolume() {
	const externalVolumeName = "nfs-images"

	suite.applyConfig(newExternalVolume(externalVolumeName), newDocBackedBy(externalVolumeName))

	ctest.AssertResource(suite, testLibrary, func(status *hypervisor.ContentLibraryStatus, asrt *assert.Assertions) {
		asrt.Equal(constants.ExternalVolumePrefix+externalVolumeName, status.TypedSpec().VolumeID)
	})
}

// TestResolvesExistingVolume covers a library backed by an existing volume, which the user declares
// by pointing at a volume already on the disk.
func (suite *ContentLibrarySuite) TestResolvesExistingVolume() {
	const existingVolumeName = "found-images"

	suite.applyConfig(newExistingVolume(existingVolumeName), newDocBackedBy(existingVolumeName))

	ctest.AssertResource(suite, testLibrary, func(status *hypervisor.ContentLibraryStatus, asrt *assert.Assertions) {
		asrt.Equal(constants.ExistingVolumePrefix+existingVolumeName, status.TypedSpec().VolumeID)
	})
}

// TestMissingVolume covers a library naming a volume no document declares: there is nothing to
// resolve the name against.
func (suite *ContentLibrarySuite) TestMissingVolume() {
	suite.applyConfig(newDocBackedBy("nowhere"))

	suite.assertNotReady(`no user, existing or external volume named "nowhere" is configured`)

	ctest.AssertNoResource[*block.VolumeMountRequest](suite, testRequestID)
}

// TestReadOnlyExternalBackingVolume covers a library backed by an external volume the user declared
// read-only: a mount is read-only only while every requester asks for it, so asking for the
// read-write mount a library needs would remount the volume against what the user declared.
func (suite *ContentLibrarySuite) TestReadOnlyExternalBackingVolume() {
	const readOnlyVolumeName = "readonly-images"

	suite.applyConfig(newReadOnlyExternalVolume(readOnlyVolumeName), newDocBackedBy(readOnlyVolumeName))

	suite.assertNotReady(fmt.Sprintf("backing volume %q is configured read-only", readOnlyVolumeName))

	ctest.AssertNoResource[*block.VolumeMountRequest](suite,
		contentLibraryControllerName+"/"+testLibrary+"/"+constants.ExternalVolumePrefix+readOnlyVolumeName)
}

// TestReadOnlyExistingBackingVolume covers the same for an existing volume, the other kind which
// takes a read-only mount policy.
func (suite *ContentLibrarySuite) TestReadOnlyExistingBackingVolume() {
	const readOnlyVolumeName = "readonly-found-images"

	suite.applyConfig(newReadOnlyExistingVolume(readOnlyVolumeName), newDocBackedBy(readOnlyVolumeName))

	suite.assertNotReady(fmt.Sprintf("backing volume %q is configured read-only", readOnlyVolumeName))

	ctest.AssertNoResource[*block.VolumeMountRequest](suite,
		contentLibraryControllerName+"/"+testLibrary+"/"+constants.ExistingVolumePrefix+readOnlyVolumeName)
}

// TestRequestsMount covers the mount being asked for on the library's behalf.
func (suite *ContentLibrarySuite) TestRequestsMount() {
	suite.applyLibrary(newDoc())

	ctest.AssertResource(suite, testRequestID, func(request *block.VolumeMountRequest, asrt *assert.Assertions) {
		asrt.Equal(testVolumeID, request.TypedSpec().VolumeID)
		asrt.Equal(contentLibraryControllerName, request.TypedSpec().Requester)
		// Uploads write into the library, and a library holds image data only.
		asrt.False(request.TypedSpec().ReadOnly)
		asrt.True(request.TypedSpec().Secure)
		asrt.True(request.TypedSpec().NoExec)
	})
}

// TestBecomesReady covers the library going ready once its backing volume is mounted: the mount
// target is the library's path, and the mount is held for as long as the library uses it.
func (suite *ContentLibrarySuite) TestBecomesReady() {
	target := suite.T().TempDir()

	suite.applyLibrary(newDoc())

	ctest.AssertResource(suite, testRequestID, func(*block.VolumeMountRequest, *assert.Assertions) {})

	suite.satisfyMount(testRequestID, testVolumeID, target, false)

	ctest.AssertResource(suite, testLibrary, func(status *hypervisor.ContentLibraryStatus, asrt *assert.Assertions) {
		asrt.True(status.TypedSpec().Ready, "error: %q", status.TypedSpec().Error)
		asrt.Equal(target, status.TypedSpec().Path)
		asrt.Empty(status.TypedSpec().Error)
	})

	suite.assertHeld(testRequestID, true)
}

// TestSweepsStagedUploads covers the leftovers of interrupted uploads being cleared on the way to
// ready, which is the only moment nothing can be staged.
func (suite *ContentLibrarySuite) TestSweepsStagedUploads() {
	target := suite.T().TempDir()

	staged := filepath.Join(target, ".image"+constants.ContentLibraryInflightUploadSuffix)
	suite.Require().NoError(os.WriteFile(staged, nil, 0o644))

	kept := filepath.Join(target, "image")
	suite.Require().NoError(os.WriteFile(kept, nil, 0o644))

	suite.applyLibrary(newDoc())

	ctest.AssertResource(suite, testRequestID, func(*block.VolumeMountRequest, *assert.Assertions) {})

	suite.satisfyMount(testRequestID, testVolumeID, target, false)

	ctest.AssertResource(suite, testLibrary, func(status *hypervisor.ContentLibraryStatus, asrt *assert.Assertions) {
		asrt.True(status.TypedSpec().Ready, "error: %q", status.TypedSpec().Error)
	})

	// The sweep runs just after the status is written, so the leftover goes a moment after ready.
	suite.Eventually(func() bool {
		_, err := os.Stat(staged)

		return os.IsNotExist(err)
	}, 5*time.Second, 10*time.Millisecond, "the staged upload was not swept")

	suite.FileExists(kept)
}

// TestReadOnlyMount covers another requester having left the volume mounted read-only: every upload
// would fail, so the library is better reported not ready.
func (suite *ContentLibrarySuite) TestReadOnlyMount() {
	suite.applyLibrary(newDoc())

	ctest.AssertResource(suite, testRequestID, func(*block.VolumeMountRequest, *assert.Assertions) {})

	suite.satisfyMount(testRequestID, testVolumeID, suite.T().TempDir(), true)

	suite.assertNotReady("the backing volume is mounted read-only")
	suite.assertHeld(testRequestID, false)
}

// TestMountTearingDown covers the backing volume being unmounted under a ready library: the hold has
// to be given back, or the unmount would never finish.
func (suite *ContentLibrarySuite) TestMountTearingDown() {
	suite.applyLibrary(newDoc())

	ctest.AssertResource(suite, testRequestID, func(*block.VolumeMountRequest, *assert.Assertions) {})

	suite.satisfyMount(testRequestID, testVolumeID, suite.T().TempDir(), false)

	suite.assertHeld(testRequestID, true)

	statusMD := block.NewVolumeMountStatus(block.NamespaceName, testRequestID).Metadata()

	// Pinned by a foreign finalizer, so the status stays readable while tearing down.
	suite.AddFinalizer(statusMD, "test")

	_, err := suite.State().Teardown(suite.Ctx(), statusMD)
	suite.Require().NoError(err)

	suite.assertNotReady("the backing volume is being unmounted")
	suite.assertHeld(testRequestID, false)

	suite.RemoveFinalizer(statusMD, "test")
}

// TestRepointedToAnotherVolume covers a library being backed by a different volume: the old mount is
// given back and a new one asked for.
func (suite *ContentLibrarySuite) TestRepointedToAnotherVolume() {
	const otherVolumeName = "other-images"

	suite.applyLibrary(newDoc())

	ctest.AssertResource(suite, testRequestID, func(*block.VolumeMountRequest, *assert.Assertions) {})

	suite.satisfyMount(testRequestID, testVolumeID, suite.T().TempDir(), false)

	suite.assertHeld(testRequestID, true)

	suite.updateConfig(
		newUserVolume(testVolumeName),
		newUserVolume(otherVolumeName),
		newDocBackedBy(otherVolumeName),
	)

	otherRequestID := contentLibraryControllerName + "/" + testLibrary + "/" + constants.UserVolumePrefix + otherVolumeName

	ctest.AssertResource(suite, otherRequestID, func(request *block.VolumeMountRequest, asrt *assert.Assertions) {
		asrt.Equal(constants.UserVolumePrefix+otherVolumeName, request.TypedSpec().VolumeID)
	})

	ctest.AssertNoResource[*block.VolumeMountRequest](suite, testRequestID)
	suite.assertHeld(testRequestID, false)
}

// TestRemovedFromConfig covers a library being removed: its status goes with it, and so does its
// hold on the backing volume.
func (suite *ContentLibrarySuite) TestRemovedFromConfig() {
	suite.applyLibrary(newDoc())

	ctest.AssertResource(suite, testLibrary, func(status *hypervisor.ContentLibraryStatus, asrt *assert.Assertions) {
		asrt.Equal(testVolumeID, status.TypedSpec().VolumeID)
	})

	suite.satisfyMount(testRequestID, testVolumeID, suite.T().TempDir(), false)

	suite.assertHeld(testRequestID, true)

	suite.Require().NoError(suite.State().Destroy(suite.Ctx(), config.NewMachineConfig(nil).Metadata()))

	ctest.AssertNoResource[*hypervisor.ContentLibraryStatus](suite, testLibrary)
	ctest.AssertNoResource[*block.VolumeMountRequest](suite, testRequestID)
	suite.assertHeld(testRequestID, false)
}
