// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package files_test

import (
	"maps"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	filesctrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/files"
	configconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	blockcfg "github.com/siderolabs/talos/pkg/machinery/config/types/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
	"github.com/siderolabs/talos/pkg/machinery/resources/config"
	"github.com/siderolabs/talos/pkg/machinery/resources/files"
	"github.com/siderolabs/talos/pkg/xfs"
)

const inlineDataVolumeControllerName = "files.InlineDataVolumeController"

type InlineDataVolumeSuite struct {
	ctest.DefaultSuite

	binder     *fakeBinder
	mountPoint string
}

// fakeBinder fakes bind mounts with symlinks.
type fakeBinder struct {
	mu    sync.Mutex
	binds map[string]string
	fail  bool
}

func (b *fakeBinder) Bound(target string, _ uint64) (bool, error) {
	b.mu.Lock()
	defer b.mu.Unlock()

	_, ok := b.binds[target]

	return ok, nil
}

func (b *fakeBinder) Bind(source, target string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	if b.fail {
		return os.ErrPermission
	}

	if err := os.MkdirAll(filepath.Dir(target), 0o700); err != nil {
		return err
	}

	if err := os.Symlink(source, target); err != nil {
		return err
	}

	b.binds[target] = source

	return nil
}

func (b *fakeBinder) Unbind(target string) error {
	b.mu.Lock()
	defer b.mu.Unlock()

	delete(b.binds, target)

	if err := os.Remove(target); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

func (b *fakeBinder) setFail(fail bool) {
	b.mu.Lock()
	defer b.mu.Unlock()

	b.fail = fail
}

func (b *fakeBinder) bound() map[string]string {
	b.mu.Lock()
	defer b.mu.Unlock()

	return maps.Clone(b.binds)
}

func newInlineDataVolumeDoc(name string, files map[string]string) *blockcfg.InlineDataVolumeConfigV1Alpha1 {
	doc := blockcfg.NewInlineDataVolumeConfigV1Alpha1()
	doc.MetaName = name
	doc.FilesConfig = map[string]blockcfg.InlineDataVolumeFileV1Alpha1{}

	for path, contents := range files {
		doc.FilesConfig[path] = blockcfg.InlineDataVolumeFileV1Alpha1{FileContents: contents}
	}

	return doc
}

func (suite *InlineDataVolumeSuite) setConfig(docs ...configconfig.Document) {
	cfg, err := container.New(docs...)
	suite.Require().NoError(err)

	machineConfig := config.NewMachineConfig(cfg)

	current, err := safe.StateGetByID[*config.MachineConfig](suite.Ctx(), suite.State(), config.ActiveID)
	if err != nil {
		suite.Require().True(state.IsNotFoundError(err))
		suite.Create(machineConfig)

		return
	}

	machineConfig.Metadata().SetVersion(current.Metadata().Version())
	suite.Update(machineConfig)
}

func (suite *InlineDataVolumeSuite) mount(volumeID, dir string) *block.VolumeMountStatus {
	status := block.NewVolumeMountStatus(block.NamespaceName, volumeID)
	status.TypedSpec().VolumeID = volumeID
	status.TypedSpec().Detached = true
	status.TypedSpec().SetRoot(&xfs.OSRoot{Shadow: dir})
	suite.Create(status)

	return status
}

func (suite *InlineDataVolumeSuite) assertHeld(volumeID string, held bool) {
	ctest.AssertResource(suite, volumeID, func(r *block.VolumeMountStatus, asrt *assert.Assertions) {
		asrt.Equal(held, r.Metadata().Finalizers().Has(inlineDataVolumeControllerName))
	})
}

func (suite *InlineDataVolumeSuite) TestRender() {
	dir := suite.T().TempDir()

	suite.setConfig(newInlineDataVolumeDoc("app", map[string]string{"a.conf": "a1", "sub/b.conf": "b1"}))

	ctest.AssertNoResource[*files.InlineDataVolumeStatus](suite, "app")

	status := suite.mount("i-app", dir)

	ctest.AssertResource(suite, "app", func(r *files.InlineDataVolumeStatus, asrt *assert.Assertions) {
		asrt.Equal("i-app", r.TypedSpec().VolumeID)
		asrt.Equal(2, r.TypedSpec().Files)
		asrt.NotZero(r.TypedSpec().Device)
		asrt.Equal(filesctrl.InlineDataVolumeContentsHash(testHashKey, []configconfig.InlineDataVolumeFile{
			{Path: "a.conf", Mode: 0o644, Contents: "a1"},
			{Path: "sub/b.conf", Mode: 0o644, Contents: "b1"},
		}), r.TypedSpec().ContentsHash)
	})

	suite.assertHeld("i-app", true)

	assertFile(suite.T(), dir, "a.conf", "a1", 0o644)
	assertFile(suite.T(), dir, "sub/b.conf", "b1", 0o644)

	suite.setConfig(newInlineDataVolumeDoc("app", map[string]string{"a.conf": "a2"}))

	ctest.AssertResource(suite, "app", func(r *files.InlineDataVolumeStatus, asrt *assert.Assertions) {
		asrt.Equal(1, r.TypedSpec().Files)
	})

	assertFile(suite.T(), dir, "a.conf", "a2", 0o644)
	suite.Assert().NoFileExists(filepath.Join(dir, "sub", "b.conf"))

	_, err := suite.State().Teardown(suite.Ctx(), status.Metadata())
	suite.Require().NoError(err)

	suite.assertHeld("i-app", false)
	ctest.AssertNoResource[*files.InlineDataVolumeStatus](suite, "app")

	suite.Destroy(status)

	dir2 := suite.T().TempDir()
	suite.mount("i-app", dir2)

	ctest.AssertResource(suite, "app", func(r *files.InlineDataVolumeStatus, asrt *assert.Assertions) {
		asrt.Equal(1, r.TypedSpec().Files)
	})

	assertFile(suite.T(), dir2, "a.conf", "a2", 0o644)

	suite.setConfig()

	ctest.AssertNoResource[*files.InlineDataVolumeStatus](suite, "app")
	suite.assertHeld("i-app", false)

	_, err = os.Stat(filepath.Join(dir2, "a.conf"))
	suite.Require().NoError(err)
}

func (suite *InlineDataVolumeSuite) TestMultipleVolumes() {
	dir1, dir2 := suite.T().TempDir(), suite.T().TempDir()

	suite.setConfig(
		newInlineDataVolumeDoc("one", map[string]string{"one.conf": "1"}),
		newInlineDataVolumeDoc("two", map[string]string{"two.conf": "2"}),
	)

	suite.mount("i-one", dir1)
	suite.mount("i-two", dir2)

	ctest.AssertResources(suite, []resource.ID{"one", "two"}, func(r *files.InlineDataVolumeStatus, asrt *assert.Assertions) {
		asrt.Equal(1, r.TypedSpec().Files)
	})

	assertFile(suite.T(), dir1, "one.conf", "1", 0o644)
	assertFile(suite.T(), dir2, "two.conf", "2", 0o644)

	suite.setConfig(newInlineDataVolumeDoc("two", map[string]string{"two.conf": "2"}))

	ctest.AssertNoResource[*files.InlineDataVolumeStatus](suite, "one")
	ctest.AssertResource(suite, "two", func(*files.InlineDataVolumeStatus, *assert.Assertions) {})

	suite.assertHeld("i-one", false)
	suite.assertHeld("i-two", true)
}

func (suite *InlineDataVolumeSuite) TestFailingVolumeDoesNotBlockOthers() {
	dir := suite.T().TempDir()

	suite.setConfig(
		newInlineDataVolumeDoc("broken", map[string]string{"a.conf": "a"}),
		newInlineDataVolumeDoc("good", map[string]string{"g.conf": "g"}),
	)

	// a mount without a root to write through
	broken := block.NewVolumeMountStatus(block.NamespaceName, "i-broken")
	broken.TypedSpec().VolumeID = "i-broken"
	suite.Create(broken)

	suite.mount("i-good", dir)

	ctest.AssertResource(suite, "good", func(*files.InlineDataVolumeStatus, *assert.Assertions) {})
	ctest.AssertNoResource[*files.InlineDataVolumeStatus](suite, "broken")

	assertFile(suite.T(), dir, "g.conf", "g", 0o644)

	suite.setConfig(
		newInlineDataVolumeDoc("broken", map[string]string{"a.conf": "a"}),
		newInlineDataVolumeDoc("good", map[string]string{"g.conf": "g2"}),
	)

	ctest.AssertResource(suite, "good", func(r *files.InlineDataVolumeStatus, asrt *assert.Assertions) {
		asrt.Equal(filesctrl.InlineDataVolumeContentsHash(testHashKey, []configconfig.InlineDataVolumeFile{
			{Path: "g.conf", Mode: 0o644, Contents: "g2"},
		}), r.TypedSpec().ContentsHash)
	})

	assertFile(suite.T(), dir, "g.conf", "g2", 0o644)
}

func (suite *InlineDataVolumeSuite) TestFailedRenderKeepsStatus() {
	flakyDir, goodDir := suite.T().TempDir(), suite.T().TempDir()

	suite.setConfig(
		newInlineDataVolumeDoc("flaky", map[string]string{"f.conf": "f1"}),
		newInlineDataVolumeDoc("good", map[string]string{"g.conf": "g1"}),
	)

	flaky := suite.mount("i-flaky", flakyDir)
	suite.mount("i-good", goodDir)

	hash := func(path, contents string) string {
		return filesctrl.InlineDataVolumeContentsHash(testHashKey, []configconfig.InlineDataVolumeFile{
			{Path: path, Mode: 0o644, Contents: contents},
		})
	}

	ctest.AssertResource(suite, "flaky", func(r *files.InlineDataVolumeStatus, asrt *assert.Assertions) {
		asrt.Equal(hash("f.conf", "f1"), r.TypedSpec().ContentsHash)
	})

	setReadOnly := func(readOnly bool) {
		_, err := safe.StateUpdateWithConflicts(suite.Ctx(), suite.State(), flaky.Metadata(),
			func(r *block.VolumeMountStatus) error {
				r.TypedSpec().ReadOnly = readOnly

				return nil
			})
		suite.Require().NoError(err)
	}

	// the flaky volume can't be rendered anymore
	setReadOnly(true)

	suite.setConfig(
		newInlineDataVolumeDoc("flaky", map[string]string{"f.conf": "f2"}),
		newInlineDataVolumeDoc("good", map[string]string{"g.conf": "g2"}),
	)

	// the other volume is updated
	ctest.AssertResource(suite, "good", func(r *files.InlineDataVolumeStatus, asrt *assert.Assertions) {
		asrt.Equal(hash("g.conf", "g2"), r.TypedSpec().ContentsHash)
	})

	assertFile(suite.T(), goodDir, "g.conf", "g2", 0o644)

	// the flaky volume keeps its previous files and status
	ctest.AssertResource(suite, "flaky", func(r *files.InlineDataVolumeStatus, asrt *assert.Assertions) {
		asrt.Equal(hash("f.conf", "f1"), r.TypedSpec().ContentsHash)
	})

	assertFile(suite.T(), flakyDir, "f.conf", "f1", 0o644)
	suite.assertHeld("i-flaky", true)

	// once it can be rendered again, it's updated
	setReadOnly(false)

	ctest.AssertResource(suite, "flaky", func(r *files.InlineDataVolumeStatus, asrt *assert.Assertions) {
		asrt.Equal(hash("f.conf", "f2"), r.TypedSpec().ContentsHash)
	})

	assertFile(suite.T(), flakyDir, "f.conf", "f2", 0o644)
}

func (suite *InlineDataVolumeSuite) TestOtherMountsIgnored() {
	dir, otherDir := suite.T().TempDir(), suite.T().TempDir()

	suite.setConfig(
		newInlineDataVolumeDoc("app", map[string]string{"a.conf": "a"}),
		newInlineDataVolumeDoc("other", map[string]string{"o.conf": "o"}),
	)

	suite.mount("u-app", dir)

	someoneElse := block.NewVolumeMountStatus(block.NamespaceName, "someone/i-app")
	someoneElse.TypedSpec().VolumeID = "i-app"
	someoneElse.TypedSpec().SetRoot(&xfs.OSRoot{Shadow: dir})
	suite.Create(someoneElse)

	suite.mount("i-missing", dir)

	// once this is rendered, the mounts above have been seen
	suite.mount("i-other", otherDir)

	ctest.AssertResource(suite, "other", func(*files.InlineDataVolumeStatus, *assert.Assertions) {})

	for _, id := range []resource.ID{"u-app", "someone/i-app", "i-missing"} {
		suite.assertHeld(id, false)
	}

	ctest.AssertNoResource[*files.InlineDataVolumeStatus](suite, "missing")

	entries, err := os.ReadDir(dir)
	suite.Require().NoError(err)
	suite.Assert().Empty(entries)
}

func (suite *InlineDataVolumeSuite) request(id string) *files.InlineDataVolumeMountRequest {
	request := files.NewInlineDataVolumeMountRequest(files.NamespaceName, id)
	request.TypedSpec().VolumeName = "app"
	request.TypedSpec().Requester = "test"
	suite.Create(request)

	return request
}

func (suite *InlineDataVolumeSuite) hold(id string) {
	ctest.AssertResource(suite, id, func(*files.InlineDataVolumeMountStatus, *assert.Assertions) {})

	suite.Require().NoError(suite.State().AddFinalizer(suite.Ctx(), files.NewInlineDataVolumeMountStatus(files.NamespaceName, id).Metadata(), "test"))
}

func (suite *InlineDataVolumeSuite) unhold(id string) {
	suite.Require().NoError(suite.State().RemoveFinalizer(suite.Ctx(), files.NewInlineDataVolumeMountStatus(files.NamespaceName, id).Metadata(), "test"))
}

func (suite *InlineDataVolumeSuite) release(request *files.InlineDataVolumeMountRequest) {
	_, err := suite.State().Teardown(suite.Ctx(), request.Metadata())
	suite.Require().NoError(err)

	ctest.AssertResource(suite, request.Metadata().ID(), func(r *files.InlineDataVolumeMountRequest, asrt *assert.Assertions) {
		asrt.True(r.Metadata().Finalizers().Empty())
	})

	suite.Destroy(request)
}

func (suite *InlineDataVolumeSuite) TestMounts() {
	dir := suite.T().TempDir()

	suite.setConfig(newInlineDataVolumeDoc("app", map[string]string{"a.conf": "v1"}))

	one := suite.request("one.app")
	two := suite.request("two.app")

	ctest.AssertResource(suite, "one.app", func(r *files.InlineDataVolumeMountRequest, asrt *assert.Assertions) {
		asrt.True(r.Metadata().Finalizers().Has(inlineDataVolumeControllerName))
	})
	ctest.AssertNoResource[*files.InlineDataVolumeMountStatus](suite, "one.app")

	volumeMountStatus := suite.mount("i-app", dir)

	targetOne, targetTwo := filepath.Join(suite.mountPoint, "one.app"), filepath.Join(suite.mountPoint, "two.app")

	ctest.AssertResources(suite, []resource.ID{"one.app", "two.app"}, func(r *files.InlineDataVolumeMountStatus, asrt *assert.Assertions) {
		asrt.Equal("app", r.TypedSpec().VolumeName)
		asrt.Equal(filepath.Join(suite.mountPoint, r.Metadata().ID()), r.TypedSpec().Target)
		asrt.NotZero(r.TypedSpec().Device)
		asrt.NotEmpty(r.TypedSpec().ContentsHash)
	})

	suite.Assert().Equal(map[string]string{targetOne: dir, targetTwo: dir}, suite.binder.bound())

	suite.setConfig(newInlineDataVolumeDoc("app", map[string]string{"a.conf": "v2"}))

	ctest.AssertResource(suite, "app", func(r *files.InlineDataVolumeStatus, asrt *assert.Assertions) {
		asrt.Equal(filesctrl.InlineDataVolumeContentsHash(testHashKey, []configconfig.InlineDataVolumeFile{
			{Path: "a.conf", Mode: 0o644, Contents: "v2"},
		}), r.TypedSpec().ContentsHash)
	})

	assertFile(suite.T(), targetOne, "a.conf", "v2", 0o644)
	assertFile(suite.T(), targetTwo, "a.conf", "v2", 0o644)

	suite.hold("one.app")

	_, err := suite.State().Teardown(suite.Ctx(), one.Metadata())
	suite.Require().NoError(err)

	ctest.AssertResource(suite, "one.app", func(r *files.InlineDataVolumeMountStatus, asrt *assert.Assertions) {
		asrt.Equal(resource.PhaseTearingDown, r.Metadata().Phase())
	})

	suite.Assert().Contains(suite.binder.bound(), targetOne)

	suite.unhold("one.app")
	suite.release(one)

	ctest.AssertNoResource[*files.InlineDataVolumeMountStatus](suite, "one.app")
	suite.Assert().Equal(map[string]string{targetTwo: dir}, suite.binder.bound())

	// the volume is no longer configured: its binds are torn down, the held one is kept
	suite.hold("two.app")
	suite.setConfig()

	ctest.AssertNoResource[*files.InlineDataVolumeStatus](suite, "app")
	ctest.AssertResource(suite, "two.app", func(r *files.InlineDataVolumeMountStatus, asrt *assert.Assertions) {
		asrt.Equal(resource.PhaseTearingDown, r.Metadata().Phase())
	})

	_, err = suite.State().Teardown(suite.Ctx(), volumeMountStatus.Metadata())
	suite.Require().NoError(err)

	suite.assertHeld("i-app", true)
	suite.Assert().Equal(map[string]string{targetTwo: dir}, suite.binder.bound())

	// the volume is released once the bind is, while the request still exists
	suite.unhold("two.app")

	ctest.AssertNoResource[*files.InlineDataVolumeMountStatus](suite, "two.app")
	suite.assertHeld("i-app", false)
	suite.Assert().Empty(suite.binder.bound())

	ctest.AssertResource(suite, "two.app", func(r *files.InlineDataVolumeMountRequest, asrt *assert.Assertions) {
		asrt.True(r.Metadata().Finalizers().Has(inlineDataVolumeControllerName))
	})

	suite.release(two)
}

func (suite *InlineDataVolumeSuite) TestVolumeUnmountTearsDownBinds() {
	dir := suite.T().TempDir()

	suite.setConfig(newInlineDataVolumeDoc("app", map[string]string{"a.conf": "v1"}))
	suite.request("one.app")
	volumeMountStatus := suite.mount("i-app", dir)

	target := filepath.Join(suite.mountPoint, "one.app")

	suite.hold("one.app")

	// the volume is being unmounted while still configured
	_, err := suite.State().Teardown(suite.Ctx(), volumeMountStatus.Metadata())
	suite.Require().NoError(err)

	ctest.AssertResource(suite, "one.app", func(r *files.InlineDataVolumeMountStatus, asrt *assert.Assertions) {
		asrt.Equal(resource.PhaseTearingDown, r.Metadata().Phase())
	})

	suite.assertHeld("i-app", true)
	suite.Assert().Equal(map[string]string{target: dir}, suite.binder.bound())

	suite.unhold("one.app")

	ctest.AssertNoResource[*files.InlineDataVolumeMountStatus](suite, "one.app")
	suite.assertHeld("i-app", false)
	suite.Assert().Empty(suite.binder.bound())

	ctest.AssertResource(suite, "one.app", func(r *files.InlineDataVolumeMountRequest, asrt *assert.Assertions) {
		asrt.True(r.Metadata().Finalizers().Has(inlineDataVolumeControllerName))
	})

	// the volume is mounted again: the request is bound again
	suite.Destroy(volumeMountStatus)

	newDir := suite.T().TempDir()
	suite.mount("i-app", newDir)

	ctest.AssertResource(suite, "one.app", func(r *files.InlineDataVolumeMountStatus, asrt *assert.Assertions) {
		asrt.Equal(resource.PhaseRunning, r.Metadata().Phase())
	})

	suite.Assert().Equal(map[string]string{target: newDir}, suite.binder.bound())
}

func (suite *InlineDataVolumeSuite) TestUnboundRequestDoesNotHoldVolume() {
	dir := suite.T().TempDir()

	suite.binder.setFail(true)

	suite.setConfig(newInlineDataVolumeDoc("app", map[string]string{"a.conf": "v1"}))
	suite.request("one.app")
	suite.mount("i-app", dir)

	ctest.AssertResource(suite, "app", func(*files.InlineDataVolumeStatus, *assert.Assertions) {})
	suite.assertHeld("i-app", true)
	ctest.AssertNoResource[*files.InlineDataVolumeMountStatus](suite, "one.app")

	suite.setConfig()

	suite.assertHeld("i-app", false)
}

func (suite *InlineDataVolumeSuite) TestMountFailureDoesNotBlockRendering() {
	dir := suite.T().TempDir()

	suite.binder.setFail(true)

	suite.setConfig(newInlineDataVolumeDoc("app", map[string]string{"a.conf": "v1"}))
	suite.request("one.app")
	suite.mount("i-app", dir)

	ctest.AssertResource(suite, "app", func(*files.InlineDataVolumeStatus, *assert.Assertions) {})
	ctest.AssertNoResource[*files.InlineDataVolumeMountStatus](suite, "one.app")

	// the volume is still rendered while its bind fails
	suite.setConfig(newInlineDataVolumeDoc("app", map[string]string{"a.conf": "v2"}))

	ctest.AssertResource(suite, "app", func(r *files.InlineDataVolumeStatus, asrt *assert.Assertions) {
		asrt.Equal(filesctrl.InlineDataVolumeContentsHash(testHashKey, []configconfig.InlineDataVolumeFile{
			{Path: "a.conf", Mode: 0o644, Contents: "v2"},
		}), r.TypedSpec().ContentsHash)
	})

	assertFile(suite.T(), dir, "a.conf", "v2", 0o644)

	// retried on the next event
	suite.binder.setFail(false)
	suite.setConfig(newInlineDataVolumeDoc("app", map[string]string{"a.conf": "v3"}))

	ctest.AssertResource(suite, "one.app", func(*files.InlineDataVolumeMountStatus, *assert.Assertions) {})
}

func (suite *InlineDataVolumeSuite) TestLeftoverTearingDownStatus() {
	dir := suite.T().TempDir()

	suite.setConfig(newInlineDataVolumeDoc("app", map[string]string{"a.conf": "v1"}))
	one := suite.request("one.app")
	suite.mount("i-app", dir)

	suite.hold("one.app")

	// request gone while status held: status left over in teardown
	suite.Require().NoError(suite.State().RemoveFinalizer(suite.Ctx(), one.Metadata(), inlineDataVolumeControllerName))
	suite.Destroy(one)

	ctest.AssertResource(suite, "one.app", func(r *files.InlineDataVolumeMountStatus, asrt *assert.Assertions) {
		asrt.Equal(resource.PhaseTearingDown, r.Metadata().Phase())
	})

	// a request with the same ID comes back
	suite.request("one.app")
	suite.unhold("one.app")

	ctest.AssertResource(suite, "one.app", func(r *files.InlineDataVolumeMountStatus, asrt *assert.Assertions) {
		asrt.Equal(resource.PhaseRunning, r.Metadata().Phase())
		asrt.Equal(filepath.Join(suite.mountPoint, "one.app"), r.TypedSpec().Target)
	})

	suite.Assert().Equal(map[string]string{filepath.Join(suite.mountPoint, "one.app"): dir}, suite.binder.bound())
}

func (suite *InlineDataVolumeSuite) TestInvalidRequestID() {
	dir := suite.T().TempDir()

	suite.setConfig(newInlineDataVolumeDoc("app", map[string]string{"a.conf": "v1"}))
	suite.request("..")
	suite.request("ok.app")
	suite.mount("i-app", dir)

	ctest.AssertResource(suite, "ok.app", func(*files.InlineDataVolumeMountStatus, *assert.Assertions) {})
	ctest.AssertNoResource[*files.InlineDataVolumeMountStatus](suite, "..")
}

func TestCheckTmpfs(t *testing.T) {
	t.Parallel()

	checkDir := func(dir string) error {
		root, err := os.OpenRoot(dir)
		require.NoError(t, err)

		defer root.Close() //nolint:errcheck

		return filesctrl.CheckTmpfs(root)
	}

	var statfs unix.Statfs_t

	require.NoError(t, unix.Statfs("/dev/shm", &statfs))

	if statfs.Type == unix.TMPFS_MAGIC {
		require.NoError(t, checkDir("/dev/shm"))
	}

	require.NoError(t, unix.Statfs(".", &statfs))

	if statfs.Type == unix.TMPFS_MAGIC {
		t.Skip("the source tree is on a tmpfs")
	}

	require.ErrorContains(t, checkDir("."), "volume root is not a tmpfs")
}

func TestInlineDataVolumeSuite(t *testing.T) {
	t.Parallel()

	s := &InlineDataVolumeSuite{}

	s.DefaultSuite = ctest.DefaultSuite{
		Timeout: 10 * time.Second,
		AfterSetup: func(suite *ctest.DefaultSuite) {
			s.binder = &fakeBinder{binds: map[string]string{}}
			s.mountPoint = filepath.Join(suite.T().TempDir(), "inline-data-volumes")

			suite.Require().NoError(suite.Runtime().RegisterController(&filesctrl.InlineDataVolumeController{
				Binder:              s.binder,
				MountPoint:          s.mountPoint,
				SkipFilesystemCheck: true,
				HashKey:             testHashKey,
			}))
		},
	}

	suite.Run(t, s)
}

func TestInlineDataVolumeRootPath(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	status := block.NewVolumeMountStatus(block.NamespaceName, "i-foo")
	status.TypedSpec().SetRoot(&xfs.OSRoot{Shadow: dir})

	// a mount attached to a path might not be the volume, e.g. a host directory
	_, err := (&filesctrl.InlineDataVolumeController{}).RootPath(status)
	require.ErrorContains(t, err, "the volume mount has no file descriptor of a detached mount")

	path, err := (&filesctrl.InlineDataVolumeController{SkipFilesystemCheck: true, HashKey: testHashKey}).RootPath(status)
	require.NoError(t, err)
	assert.Equal(t, dir, path)
}
