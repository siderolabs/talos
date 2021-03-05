// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:scopelint
package runtime_test

import (
	"testing"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime"
)

func TestSequence_String(t *testing.T) {
	tests := []struct {
		name string
		s    runtime.Sequence
		want string
	}{
		{
			name: "boot",
			s:    runtime.SequenceBoot,
			want: "boot",
		},
		{
			name: "initialize",
			s:    runtime.SequenceInitialize,
			want: "initialize",
		},
		{
			name: "shutdown",
			s:    runtime.SequenceShutdown,
			want: "shutdown",
		},
		{
			name: "upgrade",
			s:    runtime.SequenceUpgrade,
			want: "upgrade",
		},
		{
			name: "stageUpgrade",
			s:    runtime.SequenceStageUpgrade,
			want: "stageUpgrade",
		},
		{
			name: "reboot",
			s:    runtime.SequenceReboot,
			want: "reboot",
		},
		{
			name: "reset",
			s:    runtime.SequenceReset,
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
		wantSeq runtime.Sequence
		wantErr bool
	}{
		{
			name:    "boot",
			args:    args{"boot"},
			wantSeq: runtime.SequenceBoot,
			wantErr: false,
		},
		{
			name:    "initialize",
			args:    args{"initialize"},
			wantSeq: runtime.SequenceInitialize,
			wantErr: false,
		},
		{
			name:    "shutdown",
			args:    args{"shutdown"},
			wantSeq: runtime.SequenceShutdown,
			wantErr: false,
		},
		{
			name:    "upgrade",
			args:    args{"upgrade"},
			wantSeq: runtime.SequenceUpgrade,
			wantErr: false,
		},
		{
			name:    "stageUpgrade",
			args:    args{"stageUpgrade"},
			wantSeq: runtime.SequenceStageUpgrade,
			wantErr: false,
		},
		{
			name:    "reboot",
			args:    args{"reboot"},
			wantSeq: runtime.SequenceReboot,
			wantErr: false,
		},
		{
			name:    "reset",
			args:    args{"reset"},
			wantSeq: runtime.SequenceReset,
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
			gotSeq, err := runtime.ParseSequence(tt.args.s)
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
