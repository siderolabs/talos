// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package configpatcher_test

import (
	"bytes"
	"testing"

	jsonpatch "github.com/evanphx/json-patch"

	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
)

const dummyConfig = `machine:
  kubelet: {}
`

const cloudProviderPatched = `machine:
  kubelet:
    extraArgs:
      cloud-provider: external
`

func TestJSON6902(t *testing.T) {
	type args struct {
		talosMachineConfig []byte
		patchAsBytes       []byte
	}

	tests := []struct {
		name    string
		args    args
		want    []byte
		wantErr bool
	}{
		{
			name: "test add patch",
			args: args{
				talosMachineConfig: []byte(dummyConfig),
				patchAsBytes:       []byte(`[{"op": "add", "path": "/machine/kubelet/extraArgs", "value": {"cloud-provider": "external"}}]`),
			},
			want: []byte(cloudProviderPatched),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			patch, err := jsonpatch.DecodePatch(tt.args.patchAsBytes)
			if err != nil {
				t.Errorf("JSON6902 error decoding patch: %v", err)

				return
			}

			got, err := configpatcher.JSON6902(tt.args.talosMachineConfig, patch)
			if (err != nil) != tt.wantErr {
				t.Errorf("JSON6902 error: %v, but wanted: %v", err, tt.wantErr)

				return
			}

			if !bytes.Equal(got, tt.want) {
				t.Errorf("JSON6902 got: \n%v\n but wanted: \n%v", string(got), string(tt.want))
			}
		})
	}
}
