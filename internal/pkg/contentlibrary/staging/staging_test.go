// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package staging_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/internal/pkg/contentlibrary/staging"
)

// TestNameIsRecognized covers the two halves of the package agreeing: what Name builds is what
// IsName matches, which is what lets the sweep clear it.
func TestNameIsRecognized(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"image.raw", "a", "talos-v1.11.0-metal-amd64.raw.zst"} {
		staged := staging.Name(name)

		assert.True(t, staging.IsName(staged), "%q", staged)
		assert.Len(t, staged, len(name)+staging.NameOverhead)
		assert.NotEqual(t, staged, staging.Name(name), "a staged name is unique per upload")
	}
}

// TestIsNameRejects covers what must not be taken for a staged upload: library contents, and files
// which are only half the pattern.
func TestIsNameRejects(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"image.raw", "image.raw.upload", ".image.raw", "", ".", "upload"} {
		assert.False(t, staging.IsName(name), "%q", name)
	}
}
