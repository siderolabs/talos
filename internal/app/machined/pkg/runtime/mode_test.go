// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// nolint: dupl,scopelint
package runtime

import (
	"reflect"
	"testing"
)

func TestMode_String(t *testing.T) {
	tests := []struct {
		name string
		m    Mode
		want string
	}{
		{
			name: "cloud",
			m:    ModeCloud,
			want: "cloud",
		},
		{
			name: "container",
			m:    ModeContainer,
			want: "container",
		},
		{
			name: "metal",
			m:    ModeMetal,
			want: "metal",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.m.String(); got != tt.want {
				t.Errorf("Mode.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseMode(t *testing.T) {
	type args struct {
		s string
	}

	tests := []struct {
		name    string
		args    args
		wantM   Mode
		wantErr bool
	}{
		{
			name:    "cloud",
			args:    args{"cloud"},
			wantM:   ModeCloud,
			wantErr: false,
		},
		{
			name:    "container",
			args:    args{"container"},
			wantM:   ModeContainer,
			wantErr: false,
		},
		{
			name:    "metal",
			args:    args{"metal"},
			wantM:   ModeMetal,
			wantErr: false,
		},
		{
			name:    "invalid",
			args:    args{"invalid"},
			wantM:   0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotM, err := ParseMode(tt.args.s)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseMode() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(gotM, tt.wantM) {
				t.Errorf("ParseMode() = %v, want %v", gotM, tt.wantM)
			}
		})
	}
}
