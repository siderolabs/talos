// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor

import (
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	libvirtdomain "github.com/siderolabs/talos/internal/pkg/libvirt/domain"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

func mediaSpec(domainXML string) *hypervisor.VirtualMachineDomainSpec {
	spec := hypervisor.NewVirtualMachineDomainSpec(hypervisor.NamespaceName, "media")
	spec.TypedSpec().DomainXML = domainXML

	return spec
}

var errTrayLocked = fmt.Errorf("%w: tray is locked", libvirtdomain.ErrMediaChange)

// Back-off doubles up to mediaRetryLimit; a new definition resets it.
func TestMediaRefusalBacksOff(t *testing.T) {
	t.Parallel()

	ctrl := &VirtualMachineController{GracefulStopRetryInterval: time.Second}
	ctrl.initState()

	now := time.Now()

	for attempt, want := range []time.Duration{
		time.Second, 2 * time.Second, 4 * time.Second, 8 * time.Second, 10 * time.Second, 10 * time.Second,
	} {
		ctrl.mediaRefused("media", "a", now)

		require.Equal(t, now.Add(want), ctrl.mediaPending["media"].next, "attempt %d", attempt+1)
	}

	pending := ctrl.mediaPending["media"]
	require.False(t, pending.due("a", now))
	require.True(t, pending.due("a", now.Add(10*time.Second)))
	require.True(t, pending.due("b", now), "a new definition is tried at once")

	ctrl.mediaRefused("media", "b", now)
	require.Equal(t, 1, ctrl.mediaPending["media"].attempts, "a new definition starts the back-off over")
}

// Only a pending change which is due wakes the controller on the retry interval.
func TestMediaRetryWorkIsOnlyWhatIsDue(t *testing.T) {
	t.Parallel()

	ctrl := &VirtualMachineController{GracefulStopRetryInterval: time.Hour}
	ctrl.initState()

	require.False(t, ctrl.hasRetryWork())

	ctrl.mediaRefused("media", "a", time.Now())
	require.False(t, ctrl.hasRetryWork(), "backing off")

	ctrl.mediaPending["media"] = mediaRetry{domainXML: "a", attempts: 1, next: time.Now().Add(-time.Second)}
	require.True(t, ctrl.hasRetryWork())
}

// Any outcome but another refusal ends the wait on the guest.
func TestMediaWaitEndsOnAnyOtherOutcome(t *testing.T) {
	t.Parallel()

	for name, outcome := range map[string]error{
		"success":       nil,
		"disk":          errDiskNotReady,
		"other failure": errors.New("boom"),
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			ctrl := &VirtualMachineController{GracefulStopRetryInterval: time.Second}
			ctrl.initState()

			logger := zaptest.NewLogger(t)
			spec := mediaSpec("a")

			require.NoError(t, ctrl.reconciled(logger, spec, errTrayLocked))
			require.Contains(t, ctrl.mediaPending, "media")

			err := ctrl.reconciled(logger, spec, outcome)
			if name == "other failure" {
				require.ErrorIs(t, err, outcome)
			} else {
				require.NoError(t, err)
			}

			require.NotContains(t, ctrl.mediaPending, "media")
			require.False(t, ctrl.hasRetryWork())
		})
	}
}
