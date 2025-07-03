// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package fipsmode_test

import (
	"crypto/fips140"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/fipsmode"
)

func TestEnabled(t *testing.T) {
	t.Parallel()

	assert.Equal(t, fips140.Enabled(), fipsmode.Enabled())

	t.Logf("fips140.Enabled() = %v", fips140.Enabled())
}

func TestStrict(t *testing.T) {
	t.Parallel()

	// guess strict mode from the environment
	godebug := os.Getenv("GODEBUG")
	shouldbeStrict := strings.Contains(godebug, "fips140=only")

	assert.Equal(t, shouldbeStrict, fipsmode.Strict())

	t.Logf("fipsmode.Strict() = %v", fipsmode.Strict())
}
