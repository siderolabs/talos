// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package poweroff_test

import (
	"testing"

	"github.com/siderolabs/talos/internal/app/poweroff"
)

func TestParseArgs(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		args   []string
		action poweroff.Action
	}{
		{
			name:   "shutdown no args",
			args:   []string{"shutdown"},
			action: poweroff.Shutdown,
		},
		{
			name:   "shutdown with reboot",
			args:   []string{"shutdown", "-r"},
			action: poweroff.Reboot,
		},
		{
			name:   "shutdown with reboot long",
			args:   []string{"shutdown", "--reboot"},
			action: poweroff.Reboot,
		},
		{
			name:   "shutdown with poweroff",
			args:   []string{"shutdown", "-P"},
			action: poweroff.Shutdown,
		},
		{
			name:   "shutdown with poweroff long",
			args:   []string{"shutdown", "--poweroff"},
			action: poweroff.Shutdown,
		},
		{
			name:   "shutdown with poweroff and reboot",
			args:   []string{"shutdown", "-h", "-r"},
			action: poweroff.Reboot,
		},
		{
			name:   "shutdown with poweroff, reboot and timer",
			args:   []string{"shutdown", "-h", "-r", "+0"},
			action: poweroff.Reboot,
		},
		{
			name:   "shutdown with poweroff and halt",
			args:   []string{"shutdown", "-h", "-H"},
			action: poweroff.Shutdown,
		},
		{
			name:   "shutdown with poweroff and halt long",
			args:   []string{"shutdown", "-h", "--halt"},
			action: poweroff.Shutdown,
		},
		{
			name:   "poweroff no args",
			args:   []string{"poweroff"},
			action: poweroff.Shutdown,
		},
		{
			name:   "poweroff with halt",
			args:   []string{"poweroff", "--halt"},
			action: poweroff.Shutdown,
		},
		{
			name:   "poweroff with poweroff",
			args:   []string{"poweroff", "-p"},
			action: poweroff.Shutdown,
		},
		{
			name:   "poweroff with poweroff long",
			args:   []string{"poweroff", "--poweroff"},
			action: poweroff.Shutdown,
		},
		{
			name:   "poweroff with reboot",
			args:   []string{"poweroff", "--reboot"},
			action: poweroff.Reboot,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			action := poweroff.ActionFromArgs(tt.args)
			if action != tt.action {
				t.Errorf("expected %q, got %q", tt.action, action)
			}
		})
	}
}
