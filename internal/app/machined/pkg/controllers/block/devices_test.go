// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block_test

import (
	"os"
	"syscall"
	"testing"

	"github.com/cosi-project/runtime/pkg/resource/rtestutils"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	blockctrls "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/block"
	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

// setTestFDLimit temporarily increases the file descriptor limit for tests
func setTestFDLimit(t *testing.T) syscall.Rlimit {
	var rLimit syscall.Rlimit

	err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		t.Logf("Warning: Failed to get file descriptor limit: %v", err)
		return rLimit
	}

	// Save original limit
	origLimit := rLimit

	// Set higher limits for the test
	rLimit.Cur = 65536
	if rLimit.Max < rLimit.Cur {
		rLimit.Max = rLimit.Cur
	}

	err = syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rLimit)
	if err != nil {
		t.Logf("Warning: Failed to increase file descriptor limit: %v", err)
	}

	return origLimit
}

// resetFDLimit restores the original file descriptor limit
func resetFDLimit(t *testing.T, origLimit syscall.Rlimit) {
	err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &origLimit)
	if err != nil {
		t.Logf("Warning: Failed to restore file descriptor limit: %v", err)
	}
}

type DevicesSuite struct {
	ctest.DefaultSuite
}

func TestDevicesSuite(t *testing.T) {
	suite.Run(t, new(DevicesSuite))
}

func (suite *DevicesSuite) TestDiscover() {
	if os.Geteuid() != 0 {
		suite.T().Skip("skipping test; must be root to use inotify")
	}

	// Increase file descriptor limits for this test
	origLimit := setTestFDLimit(suite.T())
	defer resetFDLimit(suite.T(), origLimit)

	suite.Require().NoError(suite.Runtime().RegisterController(&blockctrls.DevicesController{}))

	// these devices should always exist on Linux
	rtestutils.AssertResources(suite.Ctx(), suite.T(), suite.State(), []string{"loop0", "loop1"}, func(r *block.Device, assertions *assert.Assertions) {
		assertions.Equal("disk", r.TypedSpec().Type)
	})
}

// If you create any watcher or file, ensure you close it:
// watcher, err := inotify.NewWatcher()
// if err != nil {
//     t.Fatalf("failed to create watcher: %v", err)
// }
// defer watcher.Close()
