// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build !amd64

package profile_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/controllers/hypervisor/internal/profile"
	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
	"github.com/siderolabs/talos/pkg/machinery/resources/hypervisor"
)

func TestUnsupportedArchitecture(t *testing.T) {
	require.Empty(t, profile.Profiles())
	require.Error(t, profile.ValidateCustom("cluster", hypervisor.VirtualMachineProfileSpecSpec{
		Machine: "q35",
		CPU: hypervisor.VirtualMachineProfileCPUSpec{
			Mode: hypervisorhelpers.VirtualMachineProfileCPUModeHostModel,
		},
	}))
}
