// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package metal_test

import (
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"

	"github.com/talos-systems/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/metal"
)

func TestPopulateURLParameters(t *testing.T) {
	mockUUID := uuid.New()

	for _, tt := range []struct {
		name          string
		url           string
		expectedURL   string
		expectedError string
	}{
		{
			name:        "no uuid",
			url:         "http://example.com/metadata",
			expectedURL: "http://example.com/metadata",
		},
		{
			name:        "empty uuid",
			url:         "http://example.com/metadata?uuid=",
			expectedURL: fmt.Sprintf("http://example.com/metadata?uuid=%s", mockUUID.String()),
		},
		{
			name:        "uuid present",
			url:         "http://example.com/metadata?uuid=xyz",
			expectedURL: "http://example.com/metadata?uuid=xyz",
		},
		{
			name:        "other parameters",
			url:         "http://example.com/metadata?foo=a",
			expectedURL: "http://example.com/metadata?foo=a",
		},
		{
			name:        "multiple uuids",
			url:         "http://example.com/metadata?uuid=xyz&uuid=foo",
			expectedURL: fmt.Sprintf("http://example.com/metadata?uuid=%s", mockUUID.String()),
		},
	} {
		tt := tt

		t.Run(tt.name, func(t *testing.T) {
			output, err := metal.PopulateURLParameters(tt.url, func() (uuid.UUID, error) {
				return mockUUID, nil
			})

			if tt.expectedError != "" {
				assert.EqualError(t, err, tt.expectedError)
			} else {
				assert.Equal(t, output, tt.expectedURL)
			}
		})
	}
}
