// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package lifecycle_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/internal/app/lifecycle"
	"github.com/siderolabs/talos/pkg/machinery/api/common"
	runtimeres "github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

func TestValidateUpgradeContainerd(t *testing.T) {
	t.Parallel()

	inmem := &common.ContainerdInstance{Driver: common.ContainerDriver_CONTAINERD, Namespace: common.ContainerdNamespace_NS_SYSTEM}
	cri := &common.ContainerdInstance{Driver: common.ContainerDriver_CRI, Namespace: common.ContainerdNamespace_NS_CRI}

	for _, tt := range []struct {
		name    string
		state   runtimeres.SELinuxState
		inst    *common.ContainerdInstance
		wantErr bool
	}{
		{name: "enforcing inmem", state: runtimeres.SELinuxStateEnforcing, inst: inmem},
		{name: "enforcing cri", state: runtimeres.SELinuxStateEnforcing, inst: cri, wantErr: true},
		{name: "permissive cri", state: runtimeres.SELinuxStatePermissive, inst: cri},
		{name: "disabled cri", state: runtimeres.SELinuxStateDisabled, inst: cri},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			err := lifecycle.ValidateUpgradeContainerd(tt.state, tt.inst)
			if !tt.wantErr {
				assert.NoError(t, err)

				return
			}

			assert.Equal(t, codes.FailedPrecondition, status.Code(err))
		})
	}
}
