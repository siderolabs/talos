// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	hypervisorctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

type CloudInitISOSuite struct {
	ctest.DefaultSuite
}

func TestCloudInitISOSuite(t *testing.T) {
	t.Parallel()

	suite.Run(t, &CloudInitISOSuite{
		ctest.DefaultSuite{
			Timeout: 30 * time.Second,
			AfterSetup: func(s *ctest.DefaultSuite) {
				s.Require().NoError(s.Runtime().RegisterController(&hypervisorctrl.CloudInitISOController{State: s.State()}))
			},
		},
	})
}

func (s *CloudInitISOSuite) TestPublishesRealAssetAndCleansUp() {
	dir := s.T().TempDir()
	library := hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, "images")
	library.TypedSpec().Path = dir
	library.TypedSpec().VolumeID = "vol-a"
	library.TypedSpec().Phase = hypervisor.ContentLibraryPhaseReady
	s.Create(library)

	spec := hypervisor.NewCloudInitSpec(hypervisor.NamespaceName, "guest")
	*spec.TypedSpec() = hypervisor.CloudInitSpecSpec{
		Library:  "images",
		MetaData: "instance-id: guest\n",
		UserData: "#cloud-config\npassword: secret\n",
	}
	id := hypervisor.CloudInitStatusID("guest", *spec.TypedSpec())
	s.Create(spec)
	ctest.AssertResource(s, id, func(status *hypervisor.CloudInitStatus, a *assert.Assertions) {
		a.True(status.TypedSpec().Phase == hypervisor.CloudInitPhaseReady, status.TypedSpec().Error)
		a.Equal("images", status.TypedSpec().Library)
		a.Equal("vol-a", status.TypedSpec().VolumeID)
		a.Equal(dir, status.TypedSpec().Path)
		a.NotEmpty(status.TypedSpec().Digest)
		a.NotZero(status.TypedSpec().SizeBytes)
		a.NotContains(status.TypedSpec().Error, "secret")
		_, err := os.Stat(filepath.Join(dir, status.TypedSpec().Name))
		a.NoError(err)
	})
	s.Destroy(spec)
	ctest.AssertNoResource[*hypervisor.CloudInitStatus](s, id)
	// The private ownership directory is retained for subsequent incarnations.
	entries, err := os.ReadDir(dir)
	s.Require().NoError(err)
	s.Require().Len(entries, 1)
	s.Require().Equal(".cloud-init-owners", entries[0].Name())

	owners, err := os.ReadDir(filepath.Join(dir, ".cloud-init-owners"))
	s.Require().NoError(err)
	s.Require().Empty(owners)
}

func (s *CloudInitISOSuite) TestMissingLibraryPublishesWaitingStatus() {
	spec := hypervisor.NewCloudInitSpec(hypervisor.NamespaceName, "waiting")
	*spec.TypedSpec() = hypervisor.CloudInitSpecSpec{
		Library:  "images",
		UserData: "secret",
	}
	id := hypervisor.CloudInitStatusID("waiting", *spec.TypedSpec())
	s.Create(spec)
	ctest.AssertResource(s, id, func(status *hypervisor.CloudInitStatus, a *assert.Assertions) {
		a.False(status.TypedSpec().Phase == hypervisor.CloudInitPhaseReady)
		a.NotEmpty(status.TypedSpec().Error)
		a.NotContains(status.TypedSpec().Error, "secret")
		a.Equal(spec.TypedSpec().InputDigest(), status.TypedSpec().InputDigest)
	})

	dir := s.T().TempDir()
	library := hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, "images")
	library.TypedSpec().Path = dir
	library.TypedSpec().VolumeID = "vol-a"
	library.TypedSpec().Phase = hypervisor.ContentLibraryPhaseReady
	s.Create(library)
	ctest.AssertResource(s, id, func(status *hypervisor.CloudInitStatus, a *assert.Assertions) {
		a.True(status.TypedSpec().Phase == hypervisor.CloudInitPhaseReady, status.TypedSpec().Error)
	})
}

func (s *CloudInitISOSuite) TestRetiresWhileLibraryIsTearingDown() {
	dir := s.T().TempDir()
	library := hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, "images")
	library.TypedSpec().Path = dir
	library.TypedSpec().VolumeID = "vol-a"
	library.TypedSpec().Phase = hypervisor.ContentLibraryPhaseReady
	s.Create(library)

	spec := hypervisor.NewCloudInitSpec(hypervisor.NamespaceName, "leaving")
	*spec.TypedSpec() = hypervisor.CloudInitSpecSpec{
		Library: "images",
	}
	id := hypervisor.CloudInitStatusID("leaving", *spec.TypedSpec())
	s.Create(spec)
	ctest.AssertResource(s, id, func(status *hypervisor.CloudInitStatus, a *assert.Assertions) {
		a.True(status.TypedSpec().Phase == hypervisor.CloudInitPhaseReady, status.TypedSpec().Error)
	})
	_, err := s.State().Teardown(s.Ctx(), library.Metadata())
	s.Require().NoError(err)
	s.Destroy(spec)
	ctest.AssertNoResource[*hypervisor.CloudInitStatus](s, id)
	ctest.AssertResource(s, "images", func(current *hypervisor.ContentLibraryStatus, a *assert.Assertions) {
		a.False(current.Metadata().Finalizers().Has("hypervisor.CloudInitISOController"))
	})
}

func (s *CloudInitISOSuite) TestRetiresOwnedISOWhenLibraryTearsDownWithStaleReadyAndSpecRemains() {
	dir := s.T().TempDir()
	library := hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, "images")
	library.TypedSpec().Path = dir
	library.TypedSpec().VolumeID = "vol-a"
	library.TypedSpec().Phase = hypervisor.ContentLibraryPhaseReady
	s.Create(library)

	spec := hypervisor.NewCloudInitSpec(hypervisor.NamespaceName, "retained")
	*spec.TypedSpec() = hypervisor.CloudInitSpecSpec{
		Library: "images",
	}
	id := hypervisor.CloudInitStatusID(spec.Metadata().ID(), *spec.TypedSpec())
	s.Create(spec)
	ctest.AssertResource(s, id, func(status *hypervisor.CloudInitStatus, a *assert.Assertions) {
		a.True(status.TypedSpec().Phase == hypervisor.CloudInitPhaseReady, status.TypedSpec().Error)
	})

	status, err := safe.StateGetByID[*hypervisor.CloudInitStatus](s.Ctx(), s.State(), id)
	s.Require().NoError(err)

	asset := filepath.Join(dir, status.TypedSpec().Name)
	_, err = s.State().Teardown(s.Ctx(), library.Metadata())
	s.Require().NoError(err)

	current, err := safe.StateGetByID[*hypervisor.ContentLibraryStatus](s.Ctx(), s.State(), "images")
	s.Require().NoError(err)
	s.Require().Equal(resource.PhaseTearingDown, current.Metadata().Phase())
	s.Require().Equal(hypervisor.ContentLibraryPhaseReady, current.TypedSpec().Phase, "teardown must exercise stale Ready")

	s.Require().Eventually(func() bool {
		_, statErr := os.Stat(asset)
		currentLibrary, getErr := safe.StateGetByID[*hypervisor.ContentLibraryStatus](s.Ctx(), s.State(), "images")
		libraryReleased := state.IsNotFoundError(getErr) || (getErr == nil && !currentLibrary.Metadata().Finalizers().Has("hypervisor.CloudInitISOController"))

		return os.IsNotExist(statErr) && libraryReleased
	}, 3*time.Second, 10*time.Millisecond, "owned ISO and library hold must retire while the seed spec remains")
	ctest.AssertResource(s, id, func(currentStatus *hypervisor.CloudInitStatus, a *assert.Assertions) {
		a.Equal(hypervisor.CloudInitPhaseNotReady, currentStatus.TypedSpec().Phase)
		a.Empty(currentStatus.TypedSpec().Path)
	})
}

func (s *CloudInitISOSuite) TestHeldISOIsNotRemovedWhenReadyLibraryTearsDown() {
	dir := s.T().TempDir()
	library := hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, "images")
	library.TypedSpec().Path = dir
	library.TypedSpec().VolumeID = "vol-a"
	library.TypedSpec().Phase = hypervisor.ContentLibraryPhaseReady
	s.Create(library)

	spec := hypervisor.NewCloudInitSpec(hypervisor.NamespaceName, "held")
	*spec.TypedSpec() = hypervisor.CloudInitSpecSpec{
		Library: "images",
	}
	id := hypervisor.CloudInitStatusID(spec.Metadata().ID(), *spec.TypedSpec())
	s.Create(spec)
	ctest.AssertResource(s, id, func(status *hypervisor.CloudInitStatus, a *assert.Assertions) {
		a.True(status.TypedSpec().Phase == hypervisor.CloudInitPhaseReady, status.TypedSpec().Error)
	})

	status, err := safe.StateGetByID[*hypervisor.CloudInitStatus](s.Ctx(), s.State(), id)
	s.Require().NoError(err)

	asset := filepath.Join(dir, status.TypedSpec().Name)
	s.AddFinalizer(status.Metadata(), "test.running-vm")
	_, err = s.State().Teardown(s.Ctx(), library.Metadata())
	s.Require().NoError(err)

	s.Require().Eventually(func() bool {
		currentStatus, getErr := safe.StateGetByID[*hypervisor.CloudInitStatus](s.Ctx(), s.State(), id)

		return getErr == nil && currentStatus.Metadata().Phase() == resource.PhaseTearingDown
	}, 3*time.Second, 10*time.Millisecond)
	currentStatus, err := safe.StateGetByID[*hypervisor.CloudInitStatus](s.Ctx(), s.State(), id)
	s.Require().NoError(err)
	s.Require().Equal(status.TypedSpec().Path, currentStatus.TypedSpec().Path)

	_, err = os.Stat(asset)
	s.Require().NoError(err, "running VM still needs its ISO")
	currentLibrary, err := safe.StateGetByID[*hypervisor.ContentLibraryStatus](s.Ctx(), s.State(), "images")
	s.Require().NoError(err)
	s.Require().True(currentLibrary.Metadata().Finalizers().Has("hypervisor.CloudInitISOController"))

	s.RemoveFinalizer(status.Metadata(), "test.running-vm")
	s.Require().Eventually(func() bool {
		_, statErr := os.Stat(asset)
		currentLibrary, getErr := safe.StateGetByID[*hypervisor.ContentLibraryStatus](s.Ctx(), s.State(), "images")
		libraryReleased := state.IsNotFoundError(getErr) || (getErr == nil && !currentLibrary.Metadata().Finalizers().Has("hypervisor.CloudInitISOController"))

		return os.IsNotExist(statErr) && libraryReleased
	}, 3*time.Second, 10*time.Millisecond)
}

func (s *CloudInitISOSuite) TestRetirementWaitsForMutationClaimEnteredBeforeLibraryTeardown() {
	dir := s.T().TempDir()
	library := hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, "images")
	library.TypedSpec().Path = dir
	library.TypedSpec().VolumeID = "vol-a"
	library.TypedSpec().Phase = hypervisor.ContentLibraryPhaseReady
	s.Create(library)

	spec := hypervisor.NewCloudInitSpec(hypervisor.NamespaceName, "leaving-with-claim")
	*spec.TypedSpec() = hypervisor.CloudInitSpecSpec{
		Library: "images",
	}
	id := hypervisor.CloudInitStatusID(spec.Metadata().ID(), *spec.TypedSpec())
	s.Create(spec)
	ctest.AssertResource(s, id, func(status *hypervisor.CloudInitStatus, a *assert.Assertions) {
		a.True(status.TypedSpec().Phase == hypervisor.CloudInitPhaseReady, status.TypedSpec().Error)
	})

	status, err := safe.StateGetByID[*hypervisor.CloudInitStatus](s.Ctx(), s.State(), id)
	s.Require().NoError(err)

	asset := filepath.Join(dir, status.TypedSpec().Name)
	marker := hypervisor.ContentLibraryMutationFinalizer(status.TypedSpec().Name)
	s.Require().NoError(s.State().AddFinalizer(s.Ctx(), library.Metadata(), marker))
	_, err = s.State().Teardown(s.Ctx(), library.Metadata())
	s.Require().NoError(err)

	// API claims require PhaseRunning. Once teardown has begun, even a
	// simultaneous claimant cannot acquire another marker.
	current, err := safe.StateGetByID[*hypervisor.ContentLibraryStatus](s.Ctx(), s.State(), "images")
	s.Require().NoError(err)
	_, err = s.State().UpdateWithConflicts(s.Ctx(), current.Metadata(), func(r resource.Resource) error {
		r.Metadata().Finalizers().Add(hypervisor.ContentLibraryMutationFinalizer("another.iso"))

		return nil
	}, state.WithUpdateOwner(current.Metadata().Owner()), state.WithExpectedPhase(resource.PhaseRunning))
	s.Require().True(state.IsPhaseConflictError(err), "API-style claim after teardown: %v", err)

	s.Destroy(spec)
	s.Require().Eventually(func() bool {
		currentStatus, getErr := safe.StateGetByID[*hypervisor.CloudInitStatus](s.Ctx(), s.State(), id)

		return getErr == nil && currentStatus.Metadata().Phase() == resource.PhaseTearingDown
	}, time.Second, 10*time.Millisecond)

	_, err = os.Stat(asset)
	s.Require().NoError(err, "retirement must wait for the mutation claim to finish")
	current, err = safe.StateGetByID[*hypervisor.ContentLibraryStatus](s.Ctx(), s.State(), "images")
	s.Require().NoError(err)
	s.Require().True(current.Metadata().Finalizers().Has("hypervisor.CloudInitISOController"), "library hold must remain until cleanup")
	s.Require().NoError(s.State().RemoveFinalizer(s.Ctx(), library.Metadata(), marker))
	ctest.AssertNoResource[*hypervisor.CloudInitStatus](s, id)

	_, err = os.Stat(asset)
	s.Require().ErrorIs(err, os.ErrNotExist)
}

func (s *CloudInitISOSuite) TestInvalidLibraryPublishesSanitizedError() {
	spec := hypervisor.NewCloudInitSpec(hypervisor.NamespaceName, "invalid")
	*spec.TypedSpec() = hypervisor.CloudInitSpecSpec{
		Library:  "../escape",
		UserData: "secret",
	}
	id := hypervisor.CloudInitStatusID("invalid", *spec.TypedSpec())
	s.Create(spec)
	ctest.AssertResource(s, id, func(status *hypervisor.CloudInitStatus, a *assert.Assertions) {
		a.False(status.TypedSpec().Phase == hypervisor.CloudInitPhaseReady)
		a.Contains(status.TypedSpec().Error, "invalid content library")
		a.NotContains(status.TypedSpec().Error, "secret")
	})
}

func (s *CloudInitISOSuite) TestBackingChangeNeverDeletesOldPathThroughNewMount() {
	oldDir := s.T().TempDir()
	newDir := s.T().TempDir()
	library := hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, "images")
	library.TypedSpec().Path = oldDir
	library.TypedSpec().VolumeID = "vol-a"
	library.TypedSpec().Phase = hypervisor.ContentLibraryPhaseReady
	s.Create(library)

	spec := hypervisor.NewCloudInitSpec(hypervisor.NamespaceName, "backing")
	*spec.TypedSpec() = hypervisor.CloudInitSpecSpec{
		Library: "images",
	}
	id := hypervisor.CloudInitStatusID("backing", *spec.TypedSpec())
	s.Create(spec)
	ctest.AssertResource(s, id, func(status *hypervisor.CloudInitStatus, a *assert.Assertions) {
		a.True(status.TypedSpec().Phase == hypervisor.CloudInitPhaseReady, status.TypedSpec().Error)
	})
	status, err := safe.StateGetByID[*hypervisor.CloudInitStatus](s.Ctx(), s.State(), id)
	s.Require().NoError(err)

	oldFile := filepath.Join(oldDir, status.TypedSpec().Name)
	newFile := filepath.Join(newDir, status.TypedSpec().Name)
	s.Require().NoError(os.WriteFile(newFile, []byte("foreign"), 0o600))

	current, err := safe.StateGetByID[*hypervisor.ContentLibraryStatus](s.Ctx(), s.State(), "images")
	s.Require().NoError(err)

	current.TypedSpec().Path = newDir
	current.TypedSpec().VolumeID = "vol-b"
	s.Update(current, state.WithUpdateOwner(current.Metadata().Owner()))
	retained, err := safe.StateGetByID[*hypervisor.CloudInitStatus](s.Ctx(), s.State(), id)
	s.Require().NoError(err)
	s.Require().Equal(oldDir, retained.TypedSpec().Path)

	_, err = os.Stat(oldFile)
	s.Require().NoError(err)
	contents, err := os.ReadFile(newFile)
	s.Require().NoError(err)
	s.Require().Equal("foreign", string(contents))
}

func (s *CloudInitISOSuite) TestBrokenLibraryDoesNotBlockIndependentGuest() {
	brokenDir := filepath.Join(s.T().TempDir(), "missing")
	broken := hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, "broken")
	broken.TypedSpec().Path = brokenDir
	broken.TypedSpec().VolumeID = "vol-b"
	broken.TypedSpec().Phase = hypervisor.ContentLibraryPhaseReady
	s.Create(broken)

	goodDir := s.T().TempDir()
	good := hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, "good")
	good.TypedSpec().Path = goodDir
	good.TypedSpec().VolumeID = "vol-g"
	good.TypedSpec().Phase = hypervisor.ContentLibraryPhaseReady
	s.Create(good)

	badSpec := hypervisor.NewCloudInitSpec(hypervisor.NamespaceName, "bad")
	*badSpec.TypedSpec() = hypervisor.CloudInitSpecSpec{
		Library: "broken",
	}
	s.Create(badSpec)

	goodSpec := hypervisor.NewCloudInitSpec(hypervisor.NamespaceName, "good")
	*goodSpec.TypedSpec() = hypervisor.CloudInitSpecSpec{
		Library: "good",
	}
	s.Create(goodSpec)
	ctest.AssertResource(s, hypervisor.CloudInitStatusID("good", *goodSpec.TypedSpec()), func(status *hypervisor.CloudInitStatus, a *assert.Assertions) {
		a.True(status.TypedSpec().Phase == hypervisor.CloudInitPhaseReady, status.TypedSpec().Error)
	})
}

func (s *CloudInitISOSuite) TestRecoversLinkedISOAfterStatusPublicationFailure() {
	dir := s.T().TempDir()
	library := hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, "images")
	library.TypedSpec().Path = dir
	library.TypedSpec().VolumeID = "vol-a"
	library.TypedSpec().Phase = hypervisor.ContentLibraryPhaseReady
	s.Create(library)

	spec := hypervisor.NewCloudInitSpec(hypervisor.NamespaceName, "recover")
	*spec.TypedSpec() = hypervisor.CloudInitSpecSpec{
		Library:  "images",
		UserData: "secret",
	}
	id := hypervisor.CloudInitStatusID("recover", *spec.TypedSpec())
	s.Create(spec)
	ctest.AssertResource(s, id, func(status *hypervisor.CloudInitStatus, a *assert.Assertions) {
		a.True(status.TypedSpec().Phase == hypervisor.CloudInitPhaseReady, status.TypedSpec().Error)
	})
	status, err := safe.StateGetByID[*hypervisor.CloudInitStatus](s.Ctx(), s.State(), id)
	s.Require().NoError(err)
	before, err := os.Stat(filepath.Join(dir, status.TypedSpec().Name))
	s.Require().NoError(err)

	status.TypedSpec().Phase = hypervisor.CloudInitPhaseNotReady
	status.TypedSpec().Digest = ""
	status.TypedSpec().SizeBytes = 0
	status.TypedSpec().Error = "building cloud-init ISO"
	s.Update(status, state.WithUpdateOwner(status.Metadata().Owner()))
	currentLibrary, err := safe.StateGetByID[*hypervisor.ContentLibraryStatus](s.Ctx(), s.State(), "images")
	s.Require().NoError(err)

	currentLibrary.TypedSpec().Fingerprint = "trigger-reconciliation-after-restart"
	s.Update(currentLibrary, state.WithUpdateOwner(currentLibrary.Metadata().Owner()))
	ctest.AssertResource(s, id, func(current *hypervisor.CloudInitStatus, a *assert.Assertions) {
		a.Equal(hypervisor.CloudInitPhaseReady, current.TypedSpec().Phase, current.TypedSpec().Error)
		a.NotEmpty(current.TypedSpec().Digest)
	})

	after, err := os.Stat(filepath.Join(dir, status.TypedSpec().Name))
	s.Require().NoError(err)
	s.Require().True(os.SameFile(before, after), "recovery must reuse its own durable hard link")
}

func (s *CloudInitISOSuite) TestFailedOpenRetriesWithoutResourceEvent() {
	dir := filepath.Join(s.T().TempDir(), "not-yet-mounted")
	library := hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, "images")
	library.TypedSpec().Path = dir
	library.TypedSpec().VolumeID = "vol-a"
	library.TypedSpec().Phase = hypervisor.ContentLibraryPhaseReady
	s.Create(library)

	spec := hypervisor.NewCloudInitSpec(hypervisor.NamespaceName, "retry")
	*spec.TypedSpec() = hypervisor.CloudInitSpecSpec{
		Library:  "images",
		UserData: "secret",
	}
	id := hypervisor.CloudInitStatusID("retry", *spec.TypedSpec())
	s.Create(spec)
	ctest.AssertResource(s, id, func(status *hypervisor.CloudInitStatus, a *assert.Assertions) {
		a.False(status.TypedSpec().Phase == hypervisor.CloudInitPhaseReady)
		a.NotEmpty(status.TypedSpec().Error)
	})
	s.Require().NoError(os.Mkdir(dir, 0o700))
	ctest.AssertResource(s, id, func(status *hypervisor.CloudInitStatus, a *assert.Assertions) {
		a.True(status.TypedSpec().Phase == hypervisor.CloudInitPhaseReady, status.TypedSpec().Error)
	})
}

func (s *CloudInitISOSuite) TestForeignSameNameIsNeverAdopted() {
	dir := s.T().TempDir()
	library := hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, "images")
	library.TypedSpec().Path = dir
	library.TypedSpec().VolumeID = "vol-a"
	library.TypedSpec().Phase = hypervisor.ContentLibraryPhaseReady
	s.Create(library)

	spec := hypervisor.NewCloudInitSpec(hypervisor.NamespaceName, "foreign")
	*spec.TypedSpec() = hypervisor.CloudInitSpecSpec{
		Library:  "images",
		UserData: "secret",
	}
	id := hypervisor.CloudInitStatusID("foreign", *spec.TypedSpec())
	name := "cloud-init-" + id[len("foreign@"):] + ".iso"
	s.Require().NoError(os.WriteFile(filepath.Join(dir, name), []byte("foreign"), 0o600))
	s.Create(spec)
	ctest.AssertResource(s, id, func(status *hypervisor.CloudInitStatus, a *assert.Assertions) {
		a.False(status.TypedSpec().Phase == hypervisor.CloudInitPhaseReady)
		a.NotEmpty(status.TypedSpec().Error)
		a.NotContains(status.TypedSpec().Error, "secret")
	})

	contents, err := os.ReadFile(filepath.Join(dir, name))
	s.Require().NoError(err)
	s.Require().Equal("foreign", string(contents))
}

func (s *CloudInitISOSuite) TestChangedPublishedFileIsNotDeleted() {
	dir := s.T().TempDir()
	library := hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, "images")
	library.TypedSpec().Path = dir
	library.TypedSpec().VolumeID = "vol-a"
	library.TypedSpec().Phase = hypervisor.ContentLibraryPhaseReady
	s.Create(library)

	spec := hypervisor.NewCloudInitSpec(hypervisor.NamespaceName, "changed")
	*spec.TypedSpec() = hypervisor.CloudInitSpecSpec{
		Library:  "images",
		UserData: "original",
	}
	id := hypervisor.CloudInitStatusID("changed", *spec.TypedSpec())
	s.Create(spec)
	ctest.AssertResource(s, id, func(status *hypervisor.CloudInitStatus, a *assert.Assertions) {
		a.True(status.TypedSpec().Phase == hypervisor.CloudInitPhaseReady, status.TypedSpec().Error)
	})
	status, err := safe.StateGetByID[*hypervisor.CloudInitStatus](s.Ctx(), s.State(), id)
	s.Require().NoError(err)

	name := status.TypedSpec().Name
	s.Require().NoError(os.Remove(filepath.Join(dir, name)))
	s.Require().NoError(os.WriteFile(filepath.Join(dir, name), []byte("foreign"), 0o600))
	s.Destroy(spec)

	contents, err := os.ReadFile(filepath.Join(dir, name))
	s.Require().NoError(err)
	s.Require().Equal("foreign", string(contents))
	current, err := safe.StateGetByID[*hypervisor.CloudInitStatus](s.Ctx(), s.State(), id)
	s.Require().NoError(err)
	s.Require().Equal(name, current.TypedSpec().Name)
}

func (s *CloudInitISOSuite) TestHeldOldGenerationSurvivesUpdate() {
	dir := s.T().TempDir()
	library := hypervisor.NewContentLibraryStatus(hypervisor.NamespaceName, "images")
	library.TypedSpec().Path = dir
	library.TypedSpec().VolumeID = "vol-a"
	library.TypedSpec().Phase = hypervisor.ContentLibraryPhaseReady
	s.Create(library)

	spec := hypervisor.NewCloudInitSpec(hypervisor.NamespaceName, "guest")
	*spec.TypedSpec() = hypervisor.CloudInitSpecSpec{
		Library:  "images",
		UserData: "original",
	}
	oldID := hypervisor.CloudInitStatusID("guest", *spec.TypedSpec())
	s.Create(spec)
	ctest.AssertResource(s, oldID, func(status *hypervisor.CloudInitStatus, a *assert.Assertions) {
		a.True(status.TypedSpec().Phase == hypervisor.CloudInitPhaseReady, status.TypedSpec().Error)
	})
	old, err := safe.StateGetByID[*hypervisor.CloudInitStatus](s.Ctx(), s.State(), oldID)
	s.Require().NoError(err)
	s.AddFinalizer(old.Metadata(), "test.consumer")

	current, err := safe.StateGetByID[*hypervisor.CloudInitSpec](s.Ctx(), s.State(), "guest")
	s.Require().NoError(err)

	current.TypedSpec().UserData = "replacement"
	s.Update(current)
	newID := hypervisor.CloudInitStatusID("guest", *current.TypedSpec())
	ctest.AssertResource(s, newID, func(status *hypervisor.CloudInitStatus, a *assert.Assertions) {
		a.True(status.TypedSpec().Phase == hypervisor.CloudInitPhaseReady, status.TypedSpec().Error)
	})
	ctest.AssertResource(s, oldID, func(status *hypervisor.CloudInitStatus, a *assert.Assertions) {
		a.Equal("original", spec.TypedSpec().UserData)
		_, statErr := os.Stat(filepath.Join(dir, status.TypedSpec().Name))
		a.NoError(statErr)
	})
	s.RemoveFinalizer(old.Metadata(), "test.consumer")
	ctest.AssertNoResource[*hypervisor.CloudInitStatus](s, oldID)
}
