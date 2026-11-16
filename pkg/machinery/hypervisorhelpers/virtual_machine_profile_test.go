// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisorhelpers_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
)

func TestValidateVirtualMachineProfileTimers(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name    string
		rtc     hypervisorhelpers.VirtualMachineProfileRTCTickPolicy
		pit     hypervisorhelpers.VirtualMachineProfilePITTickPolicy
		wantErr []string
	}{
		{
			name: "catchup-delay",
			rtc:  hypervisorhelpers.VirtualMachineProfileRTCTickPolicyCatchup,
			pit:  hypervisorhelpers.VirtualMachineProfilePITTickPolicyDelay,
		},
		{
			name: "delay-discard",
			rtc:  hypervisorhelpers.VirtualMachineProfileRTCTickPolicyDelay,
			pit:  hypervisorhelpers.VirtualMachineProfilePITTickPolicyDiscard,
		},
		{
			name:    "unknown-rtc",
			rtc:     hypervisorhelpers.VirtualMachineProfileRTCTickPolicyUnknown,
			pit:     hypervisorhelpers.VirtualMachineProfilePITTickPolicyDelay,
			wantErr: []string{"timers.rtc.tickPolicy"},
		},
		{
			name:    "unknown-pit",
			rtc:     hypervisorhelpers.VirtualMachineProfileRTCTickPolicyCatchup,
			pit:     hypervisorhelpers.VirtualMachineProfilePITTickPolicyUnknown,
			wantErr: []string{"timers.pit.tickPolicy"},
		},
		{
			name:    "out-of-range",
			rtc:     hypervisorhelpers.VirtualMachineProfileRTCTickPolicy(99),
			pit:     hypervisorhelpers.VirtualMachineProfilePITTickPolicy(-1),
			wantErr: []string{"timers.rtc.tickPolicy", "timers.pit.tickPolicy"},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := hypervisorhelpers.ValidateVirtualMachineProfileTimers(test.rtc, test.pit)
			if len(test.wantErr) == 0 {
				require.NoError(t, err)

				return
			}

			require.Error(t, err)

			for _, want := range test.wantErr {
				require.ErrorContains(t, err, want)
			}
		})
	}
}
