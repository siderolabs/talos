// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package utils_test

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/imager/utils"
)

func TestSourceDateEpochTime(t *testing.T) {
	// force a non-UTC local timezone to make sure the result doesn't depend on it
	oldLocal := time.Local                    //nolint:gosmopolitan
	time.Local = time.FixedZone("TEST", 3600) //nolint:gosmopolitan

	t.Cleanup(func() { time.Local = oldLocal }) //nolint:gosmopolitan

	t.Run("set", func(t *testing.T) {
		t.Setenv("SOURCE_DATE_EPOCH", "1700000000")

		ts, err := utils.SourceDateEpochTime()
		require.NoError(t, err)

		assert.Equal(t, time.UTC, ts.Location())
		assert.Equal(t, int64(1700000000), ts.Unix())
		assert.Equal(t, "2023-11-14T22:13:20Z", ts.Format(time.RFC3339Nano))
	})

	t.Run("unset", func(t *testing.T) {
		t.Setenv("SOURCE_DATE_EPOCH", "") // registers restore on cleanup
		require.NoError(t, os.Unsetenv("SOURCE_DATE_EPOCH"))

		before := time.Now()

		ts, err := utils.SourceDateEpochTime()
		require.NoError(t, err)

		assert.Equal(t, time.UTC, ts.Location())
		assert.False(t, ts.Before(before))
		assert.False(t, ts.After(time.Now()))
	})

	t.Run("invalid", func(t *testing.T) {
		t.Setenv("SOURCE_DATE_EPOCH", "abc")

		_, err := utils.SourceDateEpochTime()
		require.Error(t, err)
	})
}
