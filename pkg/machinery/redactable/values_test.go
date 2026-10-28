// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package redactable_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/redactable"
)

func TestRedactKeyValue(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		in       string
		expected string
		redacted bool
	}{
		{
			name:     "environment variable",
			in:       "DB_PASSWORD=hunter2",
			expected: "DB_PASSWORD=" + constants.Redacted,
			redacted: true,
		},
		{
			name:     "command line flag",
			in:       "--oidc-client-secret=s3cret",
			expected: "--oidc-client-secret=" + constants.Redacted,
			redacted: true,
		},
		{
			name:     "value with an equals sign in it",
			in:       "TOKEN=a=b=c",
			expected: "TOKEN=" + constants.Redacted,
			redacted: true,
		},
		{
			name:     "no value",
			in:       "--allow-privileged",
			expected: "--allow-privileged",
		},
		{
			name:     "empty value",
			in:       "EMPTY=",
			expected: "EMPTY=",
		},
		{
			name:     "empty string",
			in:       "",
			expected: "",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			redacted, ok := redactable.RedactKeyValue(test.in)

			assert.Equal(t, test.expected, redacted)
			assert.Equal(t, test.redacted, ok)
		})
	}
}
