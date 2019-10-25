// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package retry

import (
	"reflect"
	"testing"
	"time"
)

// nolint: scopelint
func TestNewDefaultOptions(t *testing.T) {
	type args struct {
		setters []Option
	}

	tests := []struct {
		name string
		args args
		want *Options
	}{
		{
			name: "with options",
			args: args{
				setters: []Option{WithUnits(time.Millisecond)},
			},
			want: &Options{
				Units: time.Millisecond,
			},
		},
		{
			name: "default",
			args: args{
				setters: []Option{},
			},
			want: &Options{
				Units: time.Second,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := NewDefaultOptions(tt.args.setters...); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewDefaultOptions() = %v, want %v", got, tt.want)
			}
		})
	}
}
