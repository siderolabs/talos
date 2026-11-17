// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cgroup_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/pkg/cgroup"
)

// TestCheckMembership covers telling a process apart by the cgroups procfs lists for it.
func TestCheckMembership(t *testing.T) {
	t.Parallel()

	const cgroupPath = "/taloscontainers/web"

	for _, test := range []struct {
		name       string
		contents   string
		cgroupPath *string
		wantErr    string
	}{
		{name: "unified", contents: "0::/taloscontainers/web\n"},
		{name: "unified below", contents: "0::/taloscontainers/web/nested\n"},
		{name: "cgroup v1", contents: "12:pids:/system\n4:memory:/taloscontainers/web\n"},
		{
			name:     "other container",
			contents: "0::/taloscontainers/web2\n",
			wantErr:  `it runs in cgroup "/taloscontainers/web2", not "/taloscontainers/web"`,
		},
		{
			name:     "other process",
			contents: "0::/system/runtime\n",
			wantErr:  `it runs in cgroup "/system/runtime", not "/taloscontainers/web"`,
		},
		{
			name:       "no cgroup",
			contents:   "0::/system/runtime\n",
			cgroupPath: new(""),
			wantErr:    "no cgroup to check against",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), "cgroup")
			require.NoError(t, os.WriteFile(path, []byte(test.contents), 0o600))

			want := cgroupPath
			if test.cgroupPath != nil {
				want = *test.cgroupPath
			}

			err := cgroup.CheckMembership(path, want)
			if test.wantErr == "" {
				assert.NoError(t, err)
			} else {
				assert.EqualError(t, err, test.wantErr)
			}
		})
	}
}
