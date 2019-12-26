// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package log_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	metadata "google.golang.org/grpc/metadata"

	"github.com/talos-systems/talos/pkg/grpc/middleware/log"
)

func TestExtractMetadata(t *testing.T) {
	for _, test := range []struct {
		name     string
		md       metadata.MD
		expected string
	}{
		{
			name:     "empty",
			md:       metadata.MD{},
			expected: "",
		},
		{
			name:     "regular",
			md:       metadata.Pairs("foo", "bar", "one", "two", "a", "b"),
			expected: "a=b;foo=bar;one=two",
		},
		{
			name:     "sensitive",
			md:       metadata.Pairs("foo", "bar", "token", "secret"),
			expected: "foo=bar;token=<hidden>",
		},
	} {
		ctx := context.Background()
		ctx = metadata.NewIncomingContext(ctx, test.md)

		assert.Equal(t, test.expected, log.ExtractMetadata(ctx), test.name)
	}
}
