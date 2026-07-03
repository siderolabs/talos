// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:testpackage
package md

import (
	"errors"
	"testing"

	"github.com/siderolabs/go-cmd/pkg/cmd"
)

func TestSentinelFor(t *testing.T) {
	for _, tt := range []struct {
		name string
		out  string
		want error
	}{
		{name: "exists", out: "appears to be part of a raid array", want: ErrExists},
		{name: "in use", out: "Device or resource busy", want: ErrInUse},
		{name: "not found", out: "cannot open /dev/md0: No such file or directory", want: ErrNotFound},
		{name: "invalid", out: "invalid option -- nope", want: ErrInvalidArgument},
		{name: "resync", out: "performing resync/recovery and cannot be reshaped", want: ErrResync},
		{name: "resync short", out: "md: resync or recovery in progress", want: ErrResync},
		{name: "other", out: "boom", want: ErrCommand},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := sentinelFor(&cmd.ExitError{Output: []byte(tt.out), ExitCode: 1})
			if !errors.Is(got, tt.want) {
				t.Fatalf("sentinelFor(%q) = %v, want %v", tt.out, got, tt.want)
			}
		})
	}
}
