// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:scopelint,testpackage
package acpi

import (
	"testing"

	"github.com/mdlayher/genetlink"
)

func Test_parse(t *testing.T) {
	type args struct {
		msgs  []genetlink.Message
		event string
	}

	tests := []struct {
		name    string
		args    args
		want    bool
		wantErr bool
	}{
		{
			name: PowerButtonEvent,
			args: args{
				msgs: []genetlink.Message{
					{
						Header: genetlink.Header{
							Command: 1,
							Version: 1,
						},
						Data: []byte{48, 0, 1, 0, 98, 117, 116, 116, 111, 110, 47, 112, 111, 119, 101, 114, 0, 0, 0, 0, 0, 0, 0, 0, 76, 78, 88, 80, 87, 82, 66, 78, 58, 48, 48, 0, 0, 0, 0, 0, 128, 0, 0, 0, 1, 0, 0, 0},
					},
				},
				event: PowerButtonEvent,
			},
			want:    true,
			wantErr: false,
		},
		{
			name: "battery",
			args: args{
				msgs: []genetlink.Message{
					{
						Header: genetlink.Header{
							Command: 1,
							Version: 1,
						},
						Data: []byte{48, 0, 1, 0, 98, 117, 116, 116, 111, 110, 47, 112, 111, 119, 101, 114, 0, 0, 0, 0, 0, 0, 0, 0, 76, 78, 88, 80, 87, 82, 66, 78, 58, 48, 48, 0, 0, 0, 0, 0, 128, 0, 0, 0, 1, 0, 0, 0},
					},
				},
				event: "battery",
			},
			want:    false,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parse(tt.args.msgs, tt.args.event)
			if (err != nil) != tt.wantErr {
				t.Errorf("parse() error = %v, wantErr %v", err, tt.wantErr)

				return
			}

			if got != tt.want {
				t.Errorf("parse() = %v, want %v", got, tt.want)
			}
		})
	}
}
