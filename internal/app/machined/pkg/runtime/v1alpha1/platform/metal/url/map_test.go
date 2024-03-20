// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package url_test

import (
	"context"
	"testing"
	"time"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/cosi-project/runtime/pkg/state/impl/inmem"
	"github.com/cosi-project/runtime/pkg/state/impl/namespaced"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/metal/url"
)

func TestMapValues(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name          string
		variableNames []string

		preSetup      []setupFunc
		parallelSetup []setupFunc

		expected map[string]string
	}{
		{
			name: "no variables",
		},
		{
			name:          "multiple variables",
			variableNames: []string{"uuid", "mac", "hostname", "code"},
			expected: map[string]string{
				"code":     "top-secret",
				"hostname": "some-node",
				"mac":      "12:34:56:78:90:ce",
				"uuid":     "0000-0000",
			},
			preSetup: []setupFunc{
				createSysInfo("0000-0000", "12345"),
				createMac("12:34:56:78:90:ce"),
				createHostname("some-node"),
				createCode("top-secret"),
			},
		},
		{
			name:          "mixed wait variables",
			variableNames: []string{"uuid", "mac", "hostname", "code"},
			expected: map[string]string{
				"code":     "",
				"hostname": "another-node",
				"mac":      "12:34:56:78:90:ab",
				"uuid":     "0000-1234",
			},
			preSetup: []setupFunc{
				createSysInfo("0000-1234", "12345"),
				createMac("12:34:56:78:90:ab"),
				createHostname("example-node"),
			},
			parallelSetup: []setupFunc{
				sleep(time.Second),
				updateHostname("another-node"),
				sleep(time.Second / 2),
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			st := state.WrapCore(namespaced.NewState(inmem.Build))

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			for _, f := range test.preSetup {
				f(ctx, t, st)
			}

			errCh := make(chan error)

			var result map[string]string

			go func() {
				var e error

				result, e = url.MapValues(ctx, st, test.variableNames)
				errCh <- e
			}()

			for _, f := range test.parallelSetup {
				f(ctx, t, st)
			}

			err := <-errCh
			require.NoError(t, err)

			assert.Equal(t, test.expected, result)
		})
	}
}
