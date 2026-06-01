// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package bytesize_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/bytesize"
)

func TestBytesizeNoDefaultUnit(t *testing.T) {
	t.Run("empty input", func(t *testing.T) {
		bs := bytesize.New()
		assert.NoError(t, bs.Set(""))

		assert.EqualValues(t, 0, bs.Bytes())
		assert.Equal(t, "0", bs.String())

		assert.NoError(t, bs.Set("0"))
		assert.EqualValues(t, 0, bs.Bytes())
		assert.Equal(t, "0", bs.String())
	})

	t.Run("no unit specified", func(t *testing.T) {
		bs := bytesize.New()
		assert.ErrorContains(t, bs.Set("10"), "no unit specified")
	})

	t.Run("explicit unit provided", func(t *testing.T) {
		bs := bytesize.New()
		assert.NoError(t, bs.Set("0.5mb"))
		assert.Equal(t, "0.5mb", bs.String())
		assert.Equal(t, uint64(500000), bs.Bytes())
	})
}

func TestBytesizeWithDefaultUnit(t *testing.T) {
	t.Run("empty input", func(t *testing.T) {
		bs := bytesize.WithDefaultUnit("mb")
		assert.NoError(t, bs.Set(""))

		assert.EqualValues(t, 0, bs.Bytes())
		assert.Equal(t, "0", bs.String())

		assert.NoError(t, bs.Set("0"))
		assert.EqualValues(t, 0, bs.Bytes())
		assert.Equal(t, "0", bs.String())
	})

	t.Run("no unit specified", func(t *testing.T) {
		bs := bytesize.WithDefaultUnit("mb")
		assert.NoError(t, bs.Set("10"))

		assert.Equal(t, "10mb", bs.String())
		assert.EqualValues(t, 10*1000*1000, bs.Bytes())
	})

	t.Run("explicit unit provided", func(t *testing.T) {
		bs := bytesize.WithDefaultUnit("mb")
		assert.NoError(t, bs.Set("0.5gb"))

		assert.Equal(t, "0.5gb", bs.String())
		assert.EqualValues(t, 500000000, bs.Bytes())
	})
}

func TestByteSizeUnits(t *testing.T) {
	bs := bytesize.New()
	assert.NoError(t, bs.Set("3000000000b"))

	assert.EqualValues(t, 3000, bs.Megabytes())
	assert.EqualValues(t, 3, bs.Gigabytes())

	assert.EqualValues(t, 2861, bs.Mebibytetes()) // 3,000,000,000 / 1024^2 = 2861 MiB
	assert.EqualValues(t, 2, bs.Gibibytes())      // 3,000,000,000 / 1024^3 = 2 GiB
}
