// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"golang.org/x/sys/unix"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	storagectrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/storage"
	"github.com/siderolabs/talos/internal/pkg/md"
	storageres "github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

// monitorTestDevice is the fake kernel node created for the "data" array.
const monitorTestDevice = "/dev/md-test-data"

type MDMonitorReconcileSuite struct {
	ctest.DefaultSuite

	md      *fakeMDProvisioner
	control *os.File
}

func (suite *MDMonitorReconcileSuite) TestStderrEventRefreshesIdleArray() {
	const uuid = "8d84d1c8:95d9e5bc:dc9fdd7d:51f91234"

	createDisk(&suite.DefaultSuite, "nvme0n1", "/dev/nvme0n1", "nvme")
	createDisk(&suite.DefaultSuite, "nvme1n1", "/dev/nvme1n1", "nvme")

	spec := storageres.NewMDArraySpec(storageres.NamespaceName, "data")
	spec.TypedSpec().Level = storageres.MDLevelRAID1
	suite.Require().NoError(spec.TypedSpec().VolumeSelector.UnmarshalText([]byte(`disk.transport == "nvme"`)))
	suite.Create(spec)

	ctest.AssertResource(suite, "data", func(status *storageres.MDArrayStatus, asrt *assert.Assertions) {
		asrt.Equal(storageres.MDArrayPhaseRebuilding, status.TypedSpec().Status)
		asrt.Equal(string(md.SyncActionRecover), status.TypedSpec().SyncAction)
		asrt.Equal(uuid, status.TypedSpec().UUID)
		asrt.Empty(status.TypedSpec().Error)
	})
	ctest.AssertNoResource[*storageres.MDRefreshRequest](suite, storageres.RefreshID)

	// Change only the fake kernel observation, not a watched COSI input. The
	// real subprocess stderr event must traverse Monitor and the controller
	// before the refresh resource exists; the test never writes that resource.
	suite.md.mu.Lock()
	suite.md.syncAction[monitorTestDevice] = md.SyncActionIdle
	suite.md.mu.Unlock()

	_, err := suite.control.WriteString("finished\n")
	suite.Require().NoError(err)

	ctest.AssertResource(suite, storageres.RefreshID, func(rr *storageres.MDRefreshRequest, asrt *assert.Assertions) {
		asrt.Equal(1, rr.TypedSpec().Request)
	})
	ctest.AssertResource(suite, "data", func(status *storageres.MDArrayStatus, asrt *assert.Assertions) {
		asrt.Equal(storageres.MDArrayPhaseReady, status.TypedSpec().Status)
		asrt.Equal(string(md.SyncActionIdle), status.TypedSpec().SyncAction)
		asrt.Equal(uuid, status.TypedSpec().UUID)
		asrt.Equal(2, status.TypedSpec().RaidDevices)
		asrt.Equal([]string{"/dev/nvme0n1", "/dev/nvme1n1"}, status.TypedSpec().Members)
		asrt.Empty(status.TypedSpec().Error)
	})

	suite.md.mu.Lock()
	defer suite.md.mu.Unlock()

	suite.Assert().Empty(suite.md.creates)
	suite.Assert().Empty(suite.md.adds)
	suite.Assert().Empty(suite.md.grows)
}

func TestMDMonitorReconcileSuite(t *testing.T) {
	dir := t.TempDir()
	controlPath := filepath.Join(dir, "control")
	require.NoError(t, unix.Mkfifo(controlPath, 0o600))

	control, err := os.OpenFile(controlPath, os.O_RDWR, 0)
	require.NoError(t, err)
	t.Cleanup(func() { require.NoError(t, control.Close()) })
	t.Setenv("MDADM_CONTROL", controlPath)

	script := filepath.Join(dir, "mdadm")
	// A builtin read gates event emission and keeps the actual monitor child
	// alive until suite cancellation. There are no timer/sleep descendants.
	require.NoError(t, os.WriteFile(script, []byte(`#!/bin/sh
exec 3< "$MDADM_CONTROL"
read -r action <&3
test "$action" = finished || exit 17
printf 'mdadm: RebuildFinished event detected on md device /dev/md-test-data\n' >&2
read -r action <&3
`), 0o700))

	monitor, err := md.New(md.WithMdadmPath(script))
	require.NoError(t, err)

	provisioner := newFakeMDProvisioner()
	provisioner.createNode = monitorTestDevice
	provisioner.findByMember["/dev/nvme0n1"] = monitorTestDevice
	provisioner.details[monitorTestDevice] = md.Detail{
		Level:       "raid1",
		RaidDevices: 2,
		Members:     []string{"/dev/nvme0n1", "/dev/nvme1n1"},
		UUID:        "8d84d1c8:95d9e5bc:dc9fdd7d:51f91234",
	}
	provisioner.syncAction[monitorTestDevice] = md.SyncActionRecover

	s := &MDMonitorReconcileSuite{md: provisioner, control: control}
	s.DefaultSuite = ctest.DefaultSuite{
		Timeout: 5 * time.Second,
		AfterSetup: func(suite *ctest.DefaultSuite) {
			suite.Require().NoError(suite.Runtime().RegisterController(&storagectrl.MDArrayReconcileController{
				State: suite.State(),
				MD:    provisioner,
			}))
			suite.Require().NoError(suite.Runtime().RegisterController(&storagectrl.MDMonitorController{
				MD: monitor,
			}))
		},
	}

	suite.Run(t, s)
}
