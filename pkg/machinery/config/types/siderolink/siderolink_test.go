// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package siderolink_test

import (
	_ "embed"
	"net/url"
	"testing"

	"github.com/siderolabs/gen/ensure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/siderolink"
)

func TestRedact(t *testing.T) {
	t.Parallel()

	cfg := siderolink.NewConfigV1Alpha1()
	cfg.APIUrlConfig.URL = ensure.Value(url.Parse("https://siderolink.api/?jointoken=secret&user=alice"))

	assert.Equal(t, "https://siderolink.api/?jointoken=secret&user=alice", cfg.SideroLink().APIUrl().String())

	cfg.Redact("REDACTED")

	assert.Equal(t, "https://siderolink.api/?jointoken=REDACTED&user=alice", cfg.APIUrlConfig.String())
}

//go:embed testdata/document.yaml
var expectedDocument []byte

func TestMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := siderolink.NewConfigV1Alpha1()
	cfg.APIUrlConfig.URL = ensure.Value(url.Parse("https://siderolink.api/?jointoken=secret&user=alice"))

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	assert.Equal(t, expectedDocument, marshaled)
}

func TestValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *siderolink.ConfigV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "empty",
			cfg:  siderolink.NewConfigV1Alpha1,

			expectedError: "apiUrl is required",
		},
		{
			name: "wrong scheme",
			cfg: func() *siderolink.ConfigV1Alpha1 {
				cfg := siderolink.NewConfigV1Alpha1()
				cfg.APIUrlConfig.URL = ensure.Value(url.Parse("http://siderolink.api/"))

				return cfg
			},

			expectedError: "apiUrl scheme must be https:// or grpc://",
		},
		{
			name: "extra path",
			cfg: func() *siderolink.ConfigV1Alpha1 {
				cfg := siderolink.NewConfigV1Alpha1()
				cfg.APIUrlConfig.URL = ensure.Value(url.Parse("grpc://siderolink.api/path?jointoken=foo"))

				return cfg
			},

			expectedError: "apiUrl path must be empty",
		},
		{
			name: "valid",
			cfg: func() *siderolink.ConfigV1Alpha1 {
				cfg := siderolink.NewConfigV1Alpha1()
				cfg.APIUrlConfig.URL = ensure.Value(url.Parse("https://siderolink.api:434/?jointoken=foo"))

				return cfg
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			warnings, err := test.cfg().Validate(validationMode{})

			assert.Equal(t, test.expectedWarnings, warnings)

			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

type validationMode struct{}

func (validationMode) String() string {
	return ""
}

func (validationMode) RequiresInstall() bool {
	return false
}

func (validationMode) InContainer() bool {
	return false
}
