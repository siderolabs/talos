// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package url_test

import (
	"context"
	neturl "net/url"
	"testing"

	"github.com/cosi-project/runtime/pkg/state"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/machined/pkg/runtime/v1alpha1/platform/metal/url"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

func TestVariableMatches(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name        string
		url         string
		shouldMatch map[string]struct{}
	}{
		{
			name: "no matches",
			url:  "https://example.com?foo=bar",
		},
		{
			name: "legacy UUID",
			url:  "https://example.com?uuid=&foo=bar",
			shouldMatch: map[string]struct{}{
				constants.UUIDKey: {},
			},
		},
		{
			name: "UUID static",
			url:  "https://example.com?uuid=0000-0000&foo=bar",
		},
		{
			name: "more variables",
			url:  "https://example.com?uuid=${uuid}&foo=bar&serial=${serial}&mac=${mac}&hostname=fixed&hostname=${hostname}",
			shouldMatch: map[string]struct{}{
				constants.UUIDKey:         {},
				constants.SerialNumberKey: {},
				constants.MacKey:          {},
				constants.HostnameKey:     {},
			},
		},
		{
			name: "case insensitive",
			url:  "https://example.com?uuid=${UUId}&foo=bar&serial=${SeRiaL}",
			shouldMatch: map[string]struct{}{
				constants.UUIDKey:         {},
				constants.SerialNumberKey: {},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			u, err := neturl.Parse(test.url)
			require.NoError(t, err)

			for _, variable := range url.AllVariables() {
				if _, ok := test.shouldMatch[variable.Key]; ok {
					assert.True(t, variable.Matches(u.Query()))
				} else {
					assert.False(t, variable.Matches(u.Query()))
				}
			}
		})
	}
}

type mockValue struct {
	value string
}

func (v mockValue) Get() string {
	return v.value
}

func (v mockValue) RegisterWatch(context.Context, state.State, chan<- state.Event) error {
	return nil
}

func (v mockValue) EventHandler(state.Event) (bool, error) {
	return true, nil
}

func TestVariableReplace(t *testing.T) {
	t.Parallel()

	var1 := &url.Variable{
		Key:        "var1",
		MatchOnArg: true,
		Value: mockValue{
			value: "value1",
		},
	}

	var2 := &url.Variable{
		Key: "var2",
		Value: mockValue{
			value: "value2",
		},
	}

	for _, test := range []struct {
		name     string
		url      string
		expected string
	}{
		{
			name:     "no matches",
			url:      "https://example.com?foo=bar",
			expected: "https://example.com?foo=bar",
		},
		{
			name:     "legacy match",
			url:      "https://example.com?var1=&foo=bar",
			expected: "https://example.com?foo=bar&var1=value1",
		},
		{
			name:     "variable match",
			url:      "https://example.com?a=${var1}-suffix&foo=bar&b=${var2}&b=xyz&b=${var2}",
			expected: "https://example.com?a=value1-suffix&b=value2&b=xyz&b=value2&foo=bar",
		},
		{
			name:     "case insensitive",
			url:      "https://example.com?a=${VAR1}",
			expected: "https://example.com?a=value1",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			u, err := neturl.Parse(test.url)
			require.NoError(t, err)

			query := u.Query()

			for _, variable := range []*url.Variable{var1, var2} {
				variable.Replace(query)
			}

			u.RawQuery = query.Encode()

			assert.Equal(t, test.expected, u.String())
		})
	}
}
