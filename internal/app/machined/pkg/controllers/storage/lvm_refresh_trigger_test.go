// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	storagectrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/storage"
	storageres "github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

type LVMRefreshTriggerSuite struct {
	ctest.DefaultSuite
}

func (suite *LVMRefreshTriggerSuite) refreshRequest() int {
	rr, err := ctest.Get[*storageres.LVMRefreshRequest](suite,
		storageres.NewLVMRefreshRequest(storageres.NamespaceName, storageres.RefreshID).Metadata())
	suite.Require().NoError(err)

	return rr.TypedSpec().Request
}

func (suite *LVMRefreshTriggerSuite) TestCoalescesBurstIntoSingleBump() {
	// A burst of block-layer changes within the debounce window must produce
	// a single bump, not one per event.
	createDisk(&suite.DefaultSuite, "sda", "/dev/sda", "virtio")
	createDisk(&suite.DefaultSuite, "sdb", "/dev/sdb", "virtio")
	createDisk(&suite.DefaultSuite, "sdc", "/dev/sdc", "virtio")

	ctest.AssertResource(suite, storageres.RefreshID, func(rr *storageres.LVMRefreshRequest, asrt *assert.Assertions) {
		asrt.Equal(1, rr.TypedSpec().Request)
	})

	// No further events: the counter must stay put (no self-retrigger, the
	// controller does not observe its own output).
	time.Sleep(2500 * time.Millisecond)
	suite.Require().Equal(1, suite.refreshRequest())
}

func (suite *LVMRefreshTriggerSuite) TestDistinctBurstsBumpAgain() {
	createDisk(&suite.DefaultSuite, "sda", "/dev/sda", "virtio")

	ctest.AssertResource(suite, storageres.RefreshID, func(rr *storageres.LVMRefreshRequest, asrt *assert.Assertions) {
		asrt.Equal(1, rr.TypedSpec().Request)
	})

	// A later, separate block-layer change bumps the counter again.
	createDisk(&suite.DefaultSuite, "sdb", "/dev/sdb", "virtio")

	ctest.AssertResource(suite, storageres.RefreshID, func(rr *storageres.LVMRefreshRequest, asrt *assert.Assertions) {
		asrt.Equal(2, rr.TypedSpec().Request)
	})
}

func TestLVMRefreshTriggerSuite(t *testing.T) {
	t.Parallel()

	s := &LVMRefreshTriggerSuite{}

	s.DefaultSuite = ctest.DefaultSuite{
		Timeout: 15 * time.Second,
		AfterSetup: func(suite *ctest.DefaultSuite) {
			suite.Require().NoError(suite.Runtime().RegisterController(&storagectrl.LVMRefreshTriggerController{}))
		},
	}

	suite.Run(t, s)
}
