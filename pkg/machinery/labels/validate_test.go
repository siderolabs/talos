// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package labels_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/labels"
)

func TestValidate(t *testing.T) {
	for _, tt := range []struct {
		name   string
		labels map[string]string

		expectedError string
	}{
		{
			name: "empty",
		},
		{
			name: "valid",
			labels: map[string]string{
				"talos.dev/label":        "value",
				"foo":                    "bar",
				"kubernetes.io/hostname": "hostname1",
			},
		},
		{
			name: "invalid",
			labels: map[string]string{
				"345@.345/label":         "value",
				"foo_":                   "bar",
				"/foo":                   "bar",
				"a/b/c":                  "bar",
				"kubernetes.io/hostname": "hostname1_",
				strings.Repeat("a", 64):  "bar",
				"bar":                    strings.Repeat("a", 64),
			},
			expectedError: "7 errors occurred:\n\t* prefix cannot be empty: \"/foo\"\n\t* prefix \"345@.345\" is invalid: domain doesn't match required format: \"345@.345\"\n\t* invalid format: too many slashes: \"a/b/c\"\n\t* name is too long: \"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\" (limit is 63)\n\t* label value length exceeds limit of 63: \"aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa\"\n\t* name \"foo_\" is invalid\n\t* label value \"hostname1_\" is invalid\n\n", //nolint:lll
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			err := labels.Validate(tt.labels)
			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
