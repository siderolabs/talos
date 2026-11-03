// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package mdd_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/internal/app/mdd"
)

func TestResolveAddPaths(t *testing.T) {
	t.Parallel()

	outside := filepath.Join(t.TempDir(), "disk")
	require.NoError(t, os.WriteFile(outside, nil, 0o644))

	for _, test := range []struct {
		name           string
		device, member string
		code           codes.Code
	}{
		{name: "valid", device: "/dev/null", member: "/dev/zero", code: codes.OK},
		{name: "empty device", device: "", member: "/dev/zero", code: codes.InvalidArgument},
		{name: "empty member", device: "/dev/null", member: "", code: codes.InvalidArgument},
		{name: "not under /dev", device: "/dev/null", member: outside, code: codes.InvalidArgument},
		{name: "missing", device: "/dev/null", member: "/dev/does-not-exist", code: codes.NotFound},
		{name: "escapes /dev", device: "/dev/null", member: "/dev/.." + outside, code: codes.InvalidArgument},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			arrayDev, memberDev, err := mdd.ResolveAddPaths(test.device, test.member)
			assert.Equal(t, test.code, status.Code(err), "error: %v", err)

			if test.code == codes.OK {
				assert.Equal(t, "/dev/null", arrayDev)
				assert.Equal(t, "/dev/zero", memberDev)
			}
		})
	}
}
