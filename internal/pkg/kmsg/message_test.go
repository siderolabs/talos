// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kmsg_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/talos-systems/talos/internal/pkg/kmsg"
)

func mustParse(tStr string) time.Time {
	t, err := time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", tStr)
	if err != nil {
		panic(err)
	}

	return t
}

func TestParseMessage(t *testing.T) {
	for _, testCase := range []struct {
		input    string
		expected kmsg.Message
	}{
		{
			input: `7,160,424069,-;pci_root PNP0A03:00: host bridge window [io  0x0000-0x0cf7] (ignored)
 SUBSYSTEM=acpi
 DEVICE=+acpi:PNP0A03:00`,
			expected: kmsg.Message{
				Facility:       kmsg.Kern,
				Priority:       kmsg.Debug,
				SequenceNumber: 160,
				Clock:          424069,
				Timestamp:      mustParse("0001-01-01 00:00:00.424069 +0000 UTC"),
				Message:        "pci_root PNP0A03:00: host bridge window [io  0x0000-0x0cf7] (ignored)\n SUBSYSTEM=acpi\n DEVICE=+acpi:PNP0A03:00",
			},
		},
		{
			input: `6,339,5140900,-;NET: Registered protocol family 10`,
			expected: kmsg.Message{
				Facility:       kmsg.Kern,
				Priority:       kmsg.Info,
				SequenceNumber: 339,
				Clock:          5140900,
				Timestamp:      mustParse("0001-01-01 00:00:05.1409 +0000 UTC"),
				Message:        "NET: Registered protocol family 10",
			},
		},
	} {
		message, err := kmsg.ParseMessage(testCase.input, time.Time{})
		assert.NoError(t, err)

		assert.Equal(t, testCase.expected, message)
	}
}
