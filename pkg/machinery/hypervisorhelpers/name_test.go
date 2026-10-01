// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisorhelpers_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/hypervisorhelpers"
)

func TestValidateName(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string

		expectedError string
	}{
		{name: "vm-1"},
		{name: strings.Repeat("a", 63)},
		{name: "", expectedError: "name is required"},
		{name: strings.Repeat("a", 63+1), expectedError: "must be 63 characters or fewer"},
		{name: "my vm", expectedError: "can only contain ASCII letters, digits and hyphens"},
		{name: "vm_1", expectedError: "can only contain ASCII letters, digits and hyphens"},
		{name: "vm/1", expectedError: "can only contain ASCII letters, digits and hyphens"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := hypervisorhelpers.ValidateName(test.name)

			if test.expectedError == "" {
				assert.NoError(t, err)
				assert.True(t, hypervisorhelpers.ValidNameCharset(test.name))
			} else {
				assert.ErrorContains(t, err, test.expectedError)
			}
		})
	}
}
