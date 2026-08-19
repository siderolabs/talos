// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package ctrhelper_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/internal/ctrhelper"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// TestContainerdInstanceAddress pins which containerd socket each driver/namespace pair resolves to.
//
// The containerd driver is namespace-sensitive (system lives in Talos's own containerd instance,
// everything else shares the one CRI uses), unlike the CRI driver, which always addresses the CRI
// instance regardless of namespace - that asymmetry is the part most likely to regress silently.
func TestContainerdInstanceAddress(t *testing.T) {
	for _, tc := range []struct {
		name        string
		driver      common.ContainerDriver
		namespace   common.ContainerdNamespace
		wantAddress string
		wantErr     bool
	}{
		{
			name:        "containerd driver with system namespace uses the system containerd instance",
			driver:      common.ContainerDriver_CONTAINERD,
			namespace:   common.ContainerdNamespace_NS_SYSTEM,
			wantAddress: constants.SystemContainerdAddress,
		},
		{
			name:        "containerd driver with taloscontainers namespace uses the CRI containerd instance",
			driver:      common.ContainerDriver_CONTAINERD,
			namespace:   common.ContainerdNamespace_NS_TALOSCONTAINERS,
			wantAddress: constants.CRIContainerdAddress,
		},
		{
			name:        "containerd driver with cri namespace also uses the CRI containerd instance",
			driver:      common.ContainerDriver_CONTAINERD,
			namespace:   common.ContainerdNamespace_NS_CRI,
			wantAddress: constants.CRIContainerdAddress,
		},
		{
			name:        "cri driver uses the CRI containerd instance regardless of namespace",
			driver:      common.ContainerDriver_CRI,
			namespace:   common.ContainerdNamespace_NS_SYSTEM,
			wantAddress: constants.CRIContainerdAddress,
		},
		{
			name:      "unsupported driver errors",
			driver:    common.ContainerDriver(99),
			namespace: common.ContainerdNamespace_NS_SYSTEM,
			wantErr:   true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			address, err := ctrhelper.ContainerdInstanceAddress(tc.driver, tc.namespace)

			if tc.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantAddress, address)
		})
	}
}

// TestContainerdInstanceNamespace pins the raw containerd namespace name each enum value resolves to.
func TestContainerdInstanceNamespace(t *testing.T) {
	for _, tc := range []struct {
		name          string
		namespace     common.ContainerdNamespace
		wantNamespace string
		wantErr       bool
	}{
		{
			name:          "cri",
			namespace:     common.ContainerdNamespace_NS_CRI,
			wantNamespace: constants.K8sContainerdNamespace,
		},
		{
			name:          "system",
			namespace:     common.ContainerdNamespace_NS_SYSTEM,
			wantNamespace: constants.SystemContainerdNamespace,
		},
		{
			name:          "taloscontainers",
			namespace:     common.ContainerdNamespace_NS_TALOSCONTAINERS,
			wantNamespace: constants.TalosContainersContainerdNamespace,
		},
		{
			name:      "unknown namespace errors",
			namespace: common.ContainerdNamespace_NS_UNKNOWN,
			wantErr:   true,
		},
		{
			name:      "unrecognized namespace value errors",
			namespace: common.ContainerdNamespace(99),
			wantErr:   true,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			namespace, err := ctrhelper.ContainerdInstanceNamespace(tc.namespace)

			if tc.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, tc.wantNamespace, namespace)
		})
	}
}
