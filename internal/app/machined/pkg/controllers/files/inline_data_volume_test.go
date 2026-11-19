// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package files_test

import (
	"os"
	"path/filepath"
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
		asrt.Equal(filesctrl.InlineDataVolumeContentsHash([]configconfig.InlineDataVolumeFile{
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

	suite.Run(t, &InlineDataVolumeSuite{
		Timeout: 10 * time.Second,
		AfterSetup: func(suite *ctest.DefaultSuite) {
			suite.Require().NoError(suite.Runtime().RegisterController(&filesctrl.InlineDataVolumeController{SkipFilesystemCheck: true}))
		},
	})
}
