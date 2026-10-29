// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration && integration_api

package api_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"

	"github.com/siderolabs/talos/internal/integration/api"
)

func TestParseMPSPIDList(t *testing.T) {
	for name, tc := range map[string]struct {
		output  string
		want    []int
		wantErr bool
	}{
		"empty":      {output: "", want: []int{}},
		"newline":    {output: "\n", want: []int{}},
		"single":     {output: "1234\n", want: []int{1234}},
		"multiple":   {output: "1234\n5678\n", want: []int{1234, 5678}},
		"error":      {output: "Cannot find MPS control daemon process\n", wantErr: true},
		"zero":       {output: "0\n", wantErr: true},
		"mixed":      {output: "1234\nfoo\n", wantErr: true},
		"whitespace": {output: "  42  \t 43\n", want: []int{42, 43}},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := api.ParseMPSPIDList(tc.output)
			if tc.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestParsePodUIDFromCgroup(t *testing.T) {
	const uid = "0f2d6c3e-8f5a-4c1b-9b7e-3a2d1c0b9a87"

	for name, tc := range map[string]struct {
		contents string
		want     types.UID
		ok       bool
	}{
		"cgroupfs v2": {
			contents: "0::/kubepods/besteffort/pod" + uid + "/4f3c2b1a0e9d8c7b6a5f4e3d2c1b0a9f8e7d6c5b4a3f2e1d0c9b8a7f6e5d4c3b\n",
			want:     uid,
			ok:       true,
		},
		"cgroupfs nested root": {
			contents: "0::/talos/kubepods/burstable/pod" + uid + "/abc\n",
			want:     uid,
			ok:       true,
		},
		"systemd": {
			contents: "0::/kubepods.slice/kubepods-besteffort.slice/kubepods-besteffort-pod" + strings.ReplaceAll(uid, "-", "_") +
				".slice/cri-containerd-4f3c2b1a.scope\n",
			want: uid,
			ok:   true,
		},
		"cgroup v1 repeated": {
			contents: "12:memory:/kubepods/pod" + uid + "/abc\n11:cpu:/kubepods/pod" + uid + "/abc\n",
			want:     uid,
			ok:       true,
		},
		"system service": {
			contents: "0::/system/runtime\n",
		},
		"kubepods without pod": {
			contents: "0::/kubepods/besteffort\n",
		},
		"conflicting pods": {
			contents: "12:memory:/kubepods/pod" + uid + "/abc\n11:cpu:/kubepods/pod1f2d6c3e-8f5a-4c1b-9b7e-3a2d1c0b9a87/abc\n",
		},
	} {
		t.Run(name, func(t *testing.T) {
			got, ok := api.ParsePodUIDFromCgroup(tc.contents)
			require.Equal(t, tc.ok, ok)
			require.Equal(t, tc.want, got)
		})
	}
}

func TestParseMIGModeEnabled(t *testing.T) {
	for name, tc := range map[string]struct {
		output  string
		want    bool
		wantErr bool
	}{
		"disabled":      {output: "Disabled\n"},
		"not supported": {output: "[N/A]\n"},
		"multi gpu":     {output: "Disabled\n[N/A]\n"},
		"enabled":       {output: "Disabled\nEnabled\n", want: true},
		"empty":         {output: "", wantErr: true},
		"error":         {output: "NVIDIA-SMI has failed\n", wantErr: true},
	} {
		t.Run(name, func(t *testing.T) {
			got, err := api.ParseMIGModeEnabled(tc.output)
			if tc.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}
