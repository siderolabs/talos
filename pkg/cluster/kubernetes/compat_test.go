// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubernetes_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/cluster/kubernetes"
	"github.com/siderolabs/talos/pkg/machinery/api/machine"
	"github.com/siderolabs/talos/pkg/machinery/compatibility"
)

func TestGetMinimumTalosVersion(t *testing.T) {
	v100, err := compatibility.ParseTalosVersion(&machine.VersionInfo{Tag: "v1.0.0"})
	require.NoError(t, err)

	v110, err := compatibility.ParseTalosVersion(&machine.VersionInfo{Tag: "v1.1.0"})
	require.NoError(t, err)

	v120, err := compatibility.ParseTalosVersion(&machine.VersionInfo{Tag: "v1.2.0"})
	require.NoError(t, err)

	v130Dirty, err := compatibility.ParseTalosVersion(&machine.VersionInfo{Tag: "1.3.0-alpha.2-93-gcd04c3dde-dirty"})
	require.NoError(t, err)

	tests := []struct {
		name     string
		versions []kubernetes.NodeVersion
		want     *compatibility.TalosVersion
		wantErr  bool
	}{
		{
			name:     "empty",
			versions: []kubernetes.NodeVersion{},
			want:     nil,
			wantErr:  false,
		},
		{
			name: "single version",
			versions: []kubernetes.NodeVersion{
				{
					Node:    "node1",
					Version: v100,
				},
			},
			want:    v100,
			wantErr: false,
		},
		{
			name: "single dirty version",
			versions: []kubernetes.NodeVersion{
				{
					Node:    "node1",
					Version: v130Dirty,
				},
			},
			want:    v130Dirty,
			wantErr: false,
		},
		{
			name: "multiple versions, sorted",
			versions: []kubernetes.NodeVersion{
				{
					Node:    "node1",
					Version: v100,
				},
				{
					Node:    "node2",
					Version: v110,
				},
				{
					Node:    "node3",
					Version: v120,
				},
			},
			want:    v100,
			wantErr: false,
		},
		{
			name: "multiple versions, unsorted",
			versions: []kubernetes.NodeVersion{
				{
					Node:    "node2",
					Version: v110,
				},
				{
					Node:    "node1",
					Version: v100,
				},
				{
					Node:    "node3",
					Version: v120,
				},
				{
					Node:    "node4dirty",
					Version: v130Dirty,
				},
			},
			want:    v100,
			wantErr: false,
		},
		{
			name: "multiple versions, same version",
			versions: []kubernetes.NodeVersion{
				{
					Node:    "node1",
					Version: v110,
				},
				{
					Node:    "node2",
					Version: v110,
				},
			},
			want:    v110,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := kubernetes.GetMinimumTalosVersion(tt.versions)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.want, got)
			}
		})
	}
}
