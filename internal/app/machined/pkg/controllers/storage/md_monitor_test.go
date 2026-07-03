// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package storage_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/ctest"
	storagectrl "github.com/siderolabs/talos/internal/app/machined/pkg/controllers/storage"
	storageres "github.com/siderolabs/talos/pkg/machinery/resources/storage"
)

// fakeMDMonitor emits a fixed set of monitor lines, then blocks until the
// context is canceled so the controller does not busy-loop.
type fakeMDMonitor struct {
	events []string
}

func (f *fakeMDMonitor) Monitor(ctx context.Context, onEvent func(string, error)) error {
	for _, e := range f.events {
		onEvent(e, nil)
	}

	<-ctx.Done()

	return ctx.Err()
}

type MDMonitorSuite struct {
	ctest.DefaultSuite
}

func (suite *MDMonitorSuite) TestBumpsRefreshOnDetectedEvent() {
	ctest.AssertResource(suite, storageres.RefreshID, func(rr *storageres.MDRefreshRequest, asrt *assert.Assertions) {
		// Only the "event detected" line bumps; the plain message is ignored.
		asrt.Equal(1, rr.TypedSpec().Request)
	})
}

func TestMDMonitorSuite(t *testing.T) {
	t.Parallel()

	monitor := &fakeMDMonitor{
		events: []string{
			"mdadm: NewArray event detected on md device /dev/md0",
			"mdadm: monitoring started",
		},
	}

	s := &MDMonitorSuite{}

	s.DefaultSuite = ctest.DefaultSuite{
		Timeout: 5 * time.Second,
		AfterSetup: func(suite *ctest.DefaultSuite) {
			suite.Require().NoError(suite.Runtime().RegisterController(&storagectrl.MDMonitorController{
				MD: monitor,
			}))
		},
	}

	suite.Run(t, s)
}
