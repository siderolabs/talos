// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package talos

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/api/machine"
)

func TestContentLibraryDeleteRequest(t *testing.T) {
	t.Parallel()

	hash := "sha256:" + strings.Repeat("a", 64)

	for _, tc := range []struct {
		name    string
		target  string
		force   bool
		object  bool
		wantErr bool
	}{
		{name: "alias", target: "image.iso"},
		{name: "object", target: hash, object: true},
		{name: "force object", target: hash, force: true, object: true},
		{name: "force alias", target: "image.iso", force: true, wantErr: true},
		{name: "invalid digest", target: "sha256:../escape", wantErr: true},
		{name: "uppercase digest", target: "sha256:" + strings.Repeat("A", 64), wantErr: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			req, err := contentLibraryDeleteRequest("images", tc.target, tc.force)
			if tc.wantErr {
				require.Error(t, err)
				require.Nil(t, req)

				return
			}

			require.NoError(t, err)
			require.Equal(t, "images", req.GetLibraryId())
			require.Equal(t, tc.force, req.GetForce())

			encoded, err := req.MarshalVT()
			require.NoError(t, err)

			var decoded machine.ContentLibraryServiceDeleteRequest

			require.NoError(t, decoded.UnmarshalVT(encoded))
			require.Equal(t, req, &decoded)

			if tc.object {
				require.Equal(t, tc.target, req.GetSha256())
				require.Empty(t, req.GetName())
			} else {
				require.Equal(t, tc.target, req.GetName())
				require.Empty(t, req.GetSha256())
			}
		})
	}
}
