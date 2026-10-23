// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package redactable_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/grpc/metadata"

	"github.com/siderolabs/talos/pkg/machinery/redactable"
)

// incoming turns the outgoing metadata of the client context into the incoming metadata of the
// server context, the way it travels over the wire.
func incoming(ctx context.Context) context.Context {
	md, _ := metadata.FromOutgoingContext(ctx)

	return metadata.NewIncomingContext(context.Background(), md)
}

func TestModeRoundTrip(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		// modes are set one after another with WithMode, the last one wins
		modes    []redactable.Mode
		expected redactable.Mode
	}{
		{
			name:     "not set",
			expected: redactable.ModeDisabled,
		},
		{
			name:     "disabled",
			modes:    []redactable.Mode{redactable.ModeDisabled},
			expected: redactable.ModeDisabled,
		},
		{
			name:     "accept",
			modes:    []redactable.Mode{redactable.ModeAccept},
			expected: redactable.ModeAccept,
		},
		{
			name:     "enforce",
			modes:    []redactable.Mode{redactable.ModeEnforce},
			expected: redactable.ModeEnforce,
		},
		{
			name:     "overridden",
			modes:    []redactable.Mode{redactable.ModeAccept, redactable.ModeEnforce},
			expected: redactable.ModeEnforce,
		},
		{
			name:     "cleared",
			modes:    []redactable.Mode{redactable.ModeEnforce, redactable.ModeDisabled},
			expected: redactable.ModeDisabled,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			// unrelated metadata is kept as it is
			ctx := metadata.AppendToOutgoingContext(t.Context(), "nodes", "10.5.0.2")

			for _, mode := range test.modes {
				ctx = redactable.WithMode(ctx, mode) //nolint:fatcontext
			}

			md, _ := metadata.FromOutgoingContext(ctx)
			assert.Equal(t, []string{"10.5.0.2"}, md.Get("nodes"))

			mode, err := redactable.ModeFromIncomingContext(incoming(ctx))
			require.NoError(t, err)
			assert.Equal(t, test.expected, mode)
		})
	}
}

func TestModeFromIncomingContext(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name     string
		md       metadata.MD
		expected redactable.Mode
		err      string
	}{
		{
			name:     "no metadata",
			expected: redactable.ModeDisabled,
		},
		{
			name:     "no header",
			md:       metadata.Pairs("nodes", "10.5.0.2"),
			expected: redactable.ModeDisabled,
		},
		{
			name:     "empty value",
			md:       metadata.Pairs(redactable.Header, ""),
			expected: redactable.ModeDisabled,
		},
		{
			name:     "accept",
			md:       metadata.Pairs(redactable.Header, "accept"),
			expected: redactable.ModeAccept,
		},
		{
			name:     "enforce",
			md:       metadata.Pairs(redactable.Header, "enforce"),
			expected: redactable.ModeEnforce,
		},
		{
			name: "unsupported value",
			md:   metadata.Pairs(redactable.Header, "redact-everything-please"),
			err:  `unsupported "x-talos-dev-redact" value "redact-everything-please" (supported: "accept", "enforce")`,
		},
		{
			name: "multiple headers",
			md:   metadata.Pairs(redactable.Header, "accept", redactable.Header, "enforce"),
			err:  `multiple "x-talos-dev-redact" headers in the request`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctx := t.Context()

			if test.md != nil {
				ctx = metadata.NewIncomingContext(ctx, test.md)
			}

			mode, err := redactable.ModeFromIncomingContext(ctx)

			if test.err != "" {
				require.EqualError(t, err, test.err)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, test.expected, mode)
		})
	}
}
