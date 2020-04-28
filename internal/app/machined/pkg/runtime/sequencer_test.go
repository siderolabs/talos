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
			s:    SequenceBoot,
			want: "boot",
		},
		{
			name: "initialize",
			s:    SequenceInitialize,
			want: "initialize",
		},
		{
			name: "shutdown",
			s:    SequenceShutdown,
			want: "shutdown",
		},
		{
			name: "upgrade",
			s:    SequenceUpgrade,
			want: "upgrade",
		},
		{
			name: "reboot",
			s:    SequenceReboot,
			want: "reboot",
		},
		{
			name: "reset",
			s:    SequenceReset,
			want: "reset",
		},
		{
			name: "recover",
			s:    SequenceRecover,
			want: "recover",
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
			wantSeq: SequenceBoot,
			wantErr: false,
		},
		{
			name:    "initialize",
			args:    args{"initialize"},
			wantSeq: SequenceInitialize,
			wantErr: false,
		},
		{
			name:    "shutdown",
			args:    args{"shutdown"},
			wantSeq: SequenceShutdown,
			wantErr: false,
		},
		{
			name:    "upgrade",
			args:    args{"upgrade"},
			wantSeq: SequenceUpgrade,
			wantErr: false,
		},
		{
			name:    "reboot",
			args:    args{"reboot"},
			wantSeq: SequenceReboot,
			wantErr: false,
		},
		{
			name:    "reset",
			args:    args{"reset"},
			wantSeq: SequenceReset,
			wantErr: false,
		},
		{
			name:    "recover",
			args:    args{"recover"},
			wantSeq: SequenceRecover,
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
