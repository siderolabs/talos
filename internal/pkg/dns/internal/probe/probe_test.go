// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package probe_test

import (
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"github.com/siderolabs/talos/internal/pkg/dns/internal/probe"
)

func TestStopInterruptsInterval(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ignore := goleak.IgnoreCurrent()

		var (
			p      probe.Probe
			checks atomic.Int32
		)

		p.Start(time.Hour)

		p.Do(func() error {
			checks.Add(1)

			return errors.New("upstream down")
		})

		synctest.Wait()

		p.Stop()
		p.Stop()

		synctest.Wait()

		// goleak waits well below the interval, so a loop sleeping out the interval is reported
		require.NoError(t, goleak.Find(ignore), "Stop must not leave the loop waiting for the next check")
		assert.EqualValues(t, 1, checks.Load())

		// a stopped probe starts no new checks
		p.Do(func() error {
			checks.Add(1)

			return nil
		})

		synctest.Wait()

		assert.EqualValues(t, 1, checks.Load())
	})
}
