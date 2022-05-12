// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package services_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/talos-systems/talos/internal/app/machined/pkg/system/services"
	extservices "github.com/talos-systems/talos/pkg/machinery/extensions/services"
)

// TestGetOCIOptions ensures that no oci option spec is created for the options MaskedPaths and ReadonlyPaths if they are set to nil. This is backward compatible behavior.
func TestGetOCIOptions(t *testing.T) {
	// given
	svc := &services.Extension{
		Spec: &extservices.Spec{
			Container: extservices.Container{
				Security: extservices.Security{
					MaskedPaths:   nil,
					ReadonlyPaths: nil,
				},
			},
		},
	}

	// when
	actual := svc.GetOCIOptions()

	// then
	assert.Len(t, actual, 9)
}
