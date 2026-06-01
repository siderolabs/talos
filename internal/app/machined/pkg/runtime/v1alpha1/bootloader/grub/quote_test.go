// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package grub_test

import (
	"testing"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/bootloader/grub"
)

//nolint:dupl
func TestQuote(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty",
			input:    "",
			expected: "",
		},
		{
			name:     "no special characters",
			input:    "foo",
			expected: "foo",
		},
		{
			name:     "backslash",
			input:    `foo\`,
			expected: `foo\\`,
		},
		{
			name:     "escaped backslash",
			input:    `foo\$`,
			expected: `foo\\\$`,
		},
		{
			name:     "url",
			input:    "http://my-host/config.yaml?uuid=${uuid}&serial=${serial}&mac=${mac}&hostname=${hostname}",
			expected: "http://my-host/config.yaml?uuid=\\$\\{uuid\\}\\&serial=\\$\\{serial\\}\\&mac=\\$\\{mac\\}\\&hostname=\\$\\{hostname\\}",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			actual := grub.Quote(test.input)

			if actual != test.expected {
				t.Fatalf("expected %q, got %q", test.expected, actual)
			}
		})
	}
}

//nolint:dupl
func TestUnquote(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty",
			input:    "",
			expected: "",
		},
		{
			name:     "no special characters",
			input:    "foo",
			expected: "foo",
		},
		{
			name:     "backslash",
			input:    `foo\\`,
			expected: `foo\`,
		},
		{
			name:     "escaped backslash",
			input:    `foo\\\$`,
			expected: `foo\$`,
		},
		{
			name:     "url",
			input:    "http://my-host/config.yaml?uuid=\\$\\{uuid\\}\\&serial=\\$\\{serial\\}\\&mac=\\$\\{mac\\}\\&hostname=\\$\\{hostname\\}",
			expected: "http://my-host/config.yaml?uuid=${uuid}&serial=${serial}&mac=${mac}&hostname=${hostname}",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			actual := grub.Unquote(test.input)

			if actual != test.expected {
				t.Fatalf("expected %q, got %q", test.expected, actual)
			}
		})
	}
}
