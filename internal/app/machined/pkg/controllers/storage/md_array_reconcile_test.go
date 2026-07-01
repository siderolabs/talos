// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:testpackage
package storage

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMembersToAdd(t *testing.T) {
	for _, tt := range []struct {
		name     string
		disks    []string
		existing []string
		expect   []string
	}{
		{
			name:     "fresh array, none in yet",
			disks:    []string{"/dev/sda", "/dev/sdb"},
			existing: nil,
			expect:   []string{"/dev/sda", "/dev/sdb"},
		},
		{
			name:     "all members already in",
			disks:    []string{"/dev/sda", "/dev/sdb"},
			existing: []string{"/dev/sda", "/dev/sdb"},
			expect:   nil,
		},
		{
			name:     "one new disk to add",
			disks:    []string{"/dev/sda", "/dev/sdb", "/dev/sdc"},
			existing: []string{"/dev/sda", "/dev/sdb"},
			expect:   []string{"/dev/sdc"},
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expect, membersToAdd(tt.disks, tt.existing))
		})
	}
}
