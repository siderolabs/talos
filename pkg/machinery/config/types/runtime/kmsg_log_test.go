// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	_ "embed"
	"net/url"
	"testing"

	"github.com/siderolabs/gen/ensure"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/runtime"
)

//go:embed testdata/kmsglog.yaml
var expectedKmsgLogDocument []byte

func TestKmsgLogMarshalStability(t *testing.T) {
	cfg := runtime.NewKmsgLogV1Alpha1()
	cfg.MetaName = "apiSink"
	cfg.KmsgLogURL.URL = ensure.Value(url.Parse("https://kmsglog.api/logs"))

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedKmsgLogDocument, marshaled)
}

func TestKmsgLogValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *runtime.KmsgLogV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "empty",
			cfg:  runtime.NewKmsgLogV1Alpha1,

			expectedError: "name is required",
		},
		{
			name: "no URL",
			cfg: func() *runtime.KmsgLogV1Alpha1 {
				cfg := runtime.NewKmsgLogV1Alpha1()
				cfg.MetaName = "name1"

				return cfg
			},

			expectedError: "url is required",
		},
		{
			name: "wrong scheme",
			cfg: func() *runtime.KmsgLogV1Alpha1 {
				cfg := runtime.NewKmsgLogV1Alpha1()
				cfg.MetaName = "name2"
				cfg.KmsgLogURL.URL = ensure.Value(url.Parse("https://some.destination/path"))

				return cfg
			},

			expectedError: "url scheme must be tcp:// or udp://",
		},
		{
			name: "extra path",
			cfg: func() *runtime.KmsgLogV1Alpha1 {
				cfg := runtime.NewKmsgLogV1Alpha1()
				cfg.MetaName = "name5"
				cfg.KmsgLogURL.URL = ensure.Value(url.Parse("tcp://some.destination:34/path"))

				return cfg
			},

			expectedError: "url path must be empty",
		},
		{
			name: "no port",
			cfg: func() *runtime.KmsgLogV1Alpha1 {
				cfg := runtime.NewKmsgLogV1Alpha1()
				cfg.MetaName = "name6"
				cfg.KmsgLogURL.URL = ensure.Value(url.Parse("tcp://some.destination/"))

				return cfg
			},

			expectedError: "url port is required",
		},
		{
			name: "valid TCP",
			cfg: func() *runtime.KmsgLogV1Alpha1 {
				cfg := runtime.NewKmsgLogV1Alpha1()
				cfg.MetaName = "name3"
				cfg.KmsgLogURL.URL = ensure.Value(url.Parse("tcp://10.2.3.4:5000/"))

				return cfg
			},
		},
		{
			name: "valid UDP",
			cfg: func() *runtime.KmsgLogV1Alpha1 {
				cfg := runtime.NewKmsgLogV1Alpha1()
				cfg.MetaName = "name4"
				cfg.KmsgLogURL.URL = ensure.Value(url.Parse("udp://10.2.3.4:5000/"))

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
