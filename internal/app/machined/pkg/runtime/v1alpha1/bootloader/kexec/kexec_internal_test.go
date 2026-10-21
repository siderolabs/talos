// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kexec

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"golang.org/x/sys/unix"
)

func TestLoadSkipReason(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		err  error

		skipped bool
	}{
		{name: "not in the kernel", err: unix.ENOSYS, skipped: true},
		{name: "disabled via sysctl", err: unix.EPERM, skipped: true},
		{name: "busy", err: unix.EBUSY, skipped: true},
		{name: "no memory region for the kernel", err: unix.EADDRNOTAVAIL, skipped: true},
		{name: "wrapped", err: fmt.Errorf("kexec_file_load: %w", unix.EADDRNOTAVAIL), skipped: true},
		{name: "invalid image", err: unix.EINVAL},
		{name: "out of memory", err: unix.ENOMEM},
		{name: "other error", err: errors.New("boom")},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			reason := loadSkipReason(test.err)

			if test.skipped {
				assert.NotEmpty(t, reason)
			} else {
				assert.Empty(t, reason)
			}
		})
	}
}
