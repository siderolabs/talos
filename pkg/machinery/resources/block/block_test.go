// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/cosi-project/runtime/pkg/state/registry"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/resources/block"
)

func TestRegisterResource(t *testing.T) {
	ctx := t.Context()

	resources := state.WrapCore(namespaced.NewState(inmem.Build))
	resourceRegistry := registry.NewResourceRegistry(resources)

	for _, resource := range []meta.ResourceWithRD{
		&block.Device{},
		&block.DiscoveryRefreshRequest{},
		&block.DiscoveryRefreshStatus{},
		&block.DiscoveredVolume{},
		&block.Disk{},
		&block.MountRequest{},
		&block.MountStatus{},
		&block.SwapStatus{},
		&block.Symlink{},
		&block.SystemDisk{},
		&block.UserDiskConfigStatus{},
		&block.VolumeConfig{},
		&block.VolumeLifecycle{},
		&block.VolumeMountRequest{},
		&block.VolumeMountStatus{},
		&block.VolumeStatus{},
	} {
		assert.NoError(t, resourceRegistry.Register(ctx, resource))
	}
}
