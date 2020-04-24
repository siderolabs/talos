// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// nolint: scopelint
package runtime

import "testing"

func TestSequence_String(t *testing.T) {
	tests := []struct {
		name string
		s    Sequence
		want string
	}{
		{
			name: "boot",
			s:    Boot,
			want: "boot",
		},
		{
			name: "initialize",
			s:    Initialize,
			want: "initialize",
		},
		{
			name: "shutdown",
			s:    Shutdown,
			want: "shutdown",
		},
		{
			name: "upgrade",
			s:    Upgrade,
			want: "upgrade",
		},
		{
			name: "reboot",
			s:    Reboot,
			want: "reboot",
		},
		{
			name: "reset",
			s:    Reset,
			want: "reset",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.s.String(); got != tt.want {
				t.Errorf("Sequence.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseSequence(t *testing.T) {
	type args struct {
		s string
	}

	tests := []struct {
		name    string
		args    args
		wantSeq Sequence
		wantErr bool
	}{
		{
			name:    "boot",
			args:    args{"boot"},
			wantSeq: Boot,
			wantErr: false,
		},
		{
			name:    "initialize",
			args:    args{"initialize"},
			wantSeq: Initialize,
			wantErr: false,
		},
		{
			name:    "shutdown",
			args:    args{"shutdown"},
			wantSeq: Shutdown,
			wantErr: false,
		},
		{
			name:    "upgrade",
			args:    args{"upgrade"},
			wantSeq: Upgrade,
			wantErr: false,
		},
		{
			name:    "reboot",
			args:    args{"reboot"},
			wantSeq: Reboot,
			wantErr: false,
		},
		{
			name:    "reset",
			args:    args{"reset"},
			wantSeq: Reset,
			wantErr: false,
		},
		{
			name:    "invalid",
			args:    args{"invalid"},
			wantSeq: 0,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSeq, err := ParseSequence(tt.args.s)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSequence() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotSeq != tt.wantSeq {
				t.Errorf("ParseSequence() = %v, want %v", gotSeq, tt.wantSeq)
			}
		})
	}
}
