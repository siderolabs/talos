// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package install_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/cmd/installer/pkg/install"
)

func TestParseTarget(t *testing.T) {
	type args struct {
		label      string
		deviceName string
	}

	tests := map[string]struct {
		args    args
		want    *install.Target
		wantErr bool
	}{
		"EPHEMERAL": {
			args: args{
				label:      "EPHEMERAL",
				deviceName: "/dev/sda",
			},
			want: install.EphemeralTarget("/dev/sda", install.NoFilesystem),
		},
		"UNKNOWN": {
			args: args{
				label:      "UNKNOWN",
				deviceName: "/dev/sda",
			},
			wantErr: true,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := install.ParseTarget(tt.args.label, tt.args.deviceName)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseTarget() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			require.Equal(t, tt.want, got)
		})
	}
}
