// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/resource/meta"
	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/cosi-project/runtime/pkg/state/registry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/resources/etcd"
)

func TestRegisterResource(t *testing.T) {
	ctx := t.Context()

	resources := state.WrapCore(namespaced.NewState(inmem.Build))
	resourceRegistry := registry.NewResourceRegistry(resources)

	for _, resource := range []meta.ResourceWithRD{
		&etcd.PKIStatus{},
	} {
		assert.NoError(t, resourceRegistry.Register(ctx, resource))
	}
}

func TestFormatMemberID(t *testing.T) {
	tests := []struct {
		name string
		id   uint64
		str  string
	}{
		{
			name: "small id",
			id:   1,
			str:  "0000000000000001",
		},
		{
			name: "big id",
			id:   uint64(1) << 63,
			str:  "8000000000000000",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.str, etcd.FormatMemberID(tt.id))

			id, err := etcd.ParseMemberID(tt.str)
			require.NoError(t, err)
			assert.Equal(t, tt.id, id)
		})
	}
}
