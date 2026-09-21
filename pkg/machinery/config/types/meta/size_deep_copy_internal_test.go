// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Internal test: DeepCopy and Merge are about the unexported value/raw fields,
// which are deliberately not observable through the public API.
package meta

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestByteSizeDeepCopy(t *testing.T) {
	t.Parallel()

	bs := MustByteSize("-4GiB")
	cp := bs.DeepCopy()

	assert.Equal(t, bs.Value(), cp.Value())
	assert.Equal(t, bs.IsNegative(), cp.IsNegative())
	assert.Equal(t, bs.raw, cp.raw)
	assert.NotSame(t, bs.value, cp.value)
	assert.NotSame(t, &bs.raw[0], &cp.raw[0])
}

func TestPercentageSizeDeepCopy(t *testing.T) {
	t.Parallel()

	ps := *MustSize("80%").PercentageSize
	cp := ps.DeepCopy()

	assert.Equal(t, ps.Value(), cp.Value())
	assert.Equal(t, ps.raw, cp.raw)
	assert.NotSame(t, ps.value, cp.value)
	assert.NotSame(t, &ps.raw[0], &cp.raw[0])
}

func TestSizeDeepCopy(t *testing.T) {
	t.Parallel()

	for _, value := range []string{"50GiB", "80%"} {
		t.Run(value, func(t *testing.T) {
			t.Parallel()

			s := MustSize(value)
			cp := s.DeepCopy()

			assert.Equal(t, s.Value(), cp.Value())

			if s.ByteSize != nil {
				require.NotNil(t, cp.ByteSize)
				assert.NotSame(t, s.ByteSize, cp.ByteSize)
				assert.NotSame(t, &s.ByteSize.raw[0], &cp.ByteSize.raw[0])
			}

			if s.PercentageSize != nil {
				require.NotNil(t, cp.PercentageSize)
				assert.NotSame(t, s.PercentageSize, cp.PercentageSize)
				assert.NotSame(t, &s.PercentageSize.raw[0], &cp.PercentageSize.raw[0])
			}
		})
	}
}

func TestDeepCopyZero(t *testing.T) {
	t.Parallel()

	assert.True(t, ByteSize{}.DeepCopy().IsZero())
	assert.True(t, PercentageSize{}.DeepCopy().IsZero())
	assert.True(t, Size{}.DeepCopy().IsZero())
}

func TestMarshalTextDoesNotAliasRaw(t *testing.T) {
	t.Parallel()

	bs := MustByteSize("4GiB")

	raw, err := bs.MarshalText()
	require.NoError(t, err)
	require.NotEmpty(t, raw)

	raw[0] = 'X'

	assert.Equal(t, "4GiB", string(bs.raw))
}

func TestByteSizeMerge(t *testing.T) {
	t.Parallel()

	dst := MustByteSize("1MiB")
	src := MustByteSize("-4GiB")

	require.NoError(t, dst.Merge(src))

	assert.Equal(t, src.Value(), dst.Value())
	// the negative flag has to travel with the value, otherwise validation
	// of a merged-in negative size silently passes.
	assert.True(t, dst.IsNegative())
	assert.NotSame(t, src.value, dst.value)
	assert.NotSame(t, &src.raw[0], &dst.raw[0])
}

func TestByteSizeMergeZero(t *testing.T) {
	t.Parallel()

	dst := MustByteSize("1MiB")

	require.NoError(t, dst.Merge(ByteSize{}))

	assert.Equal(t, "1MiB", string(dst.raw))
	assert.Equal(t, MustByteSize("1MiB").Value(), dst.Value())
}

func TestPercentageSizeMerge(t *testing.T) {
	t.Parallel()

	dst := *MustSize("10%").PercentageSize
	src := *MustSize("80%").PercentageSize

	require.NoError(t, dst.Merge(src))

	assert.Equal(t, src.Value(), dst.Value())
	assert.NotSame(t, src.value, dst.value)
	assert.NotSame(t, &src.raw[0], &dst.raw[0])
}

func TestPercentageSizeMergeZero(t *testing.T) {
	t.Parallel()

	dst := *MustSize("10%").PercentageSize

	require.NoError(t, dst.Merge(PercentageSize{}))

	assert.Equal(t, "10%", string(dst.raw))
	assert.Equal(t, uint64(10), dst.Value())
}

func TestSizeMerge(t *testing.T) {
	t.Parallel()

	t.Run("byte over byte", func(t *testing.T) {
		t.Parallel()

		dst := MustSize("1MiB")
		src := MustSize("4GiB")

		require.NoError(t, dst.Merge(src))

		assert.Equal(t, src.Value(), dst.Value())
		assert.Nil(t, dst.PercentageSize)
		require.NotNil(t, dst.ByteSize)
		assert.NotSame(t, src.ByteSize, dst.ByteSize)
		assert.NotSame(t, &src.ByteSize.raw[0], &dst.ByteSize.raw[0])
	})

	t.Run("percentage over byte", func(t *testing.T) {
		t.Parallel()

		dst := MustSize("1MiB")
		src := MustSize("80%")

		require.NoError(t, dst.Merge(src))

		// the byte size has to go, otherwise Value() keeps reporting the size
		// the patch replaced.
		assert.Nil(t, dst.ByteSize)
		assert.Zero(t, dst.Value())

		relative, ok := dst.RelativeValue()
		assert.True(t, ok)
		assert.Equal(t, uint64(80), relative)

		require.NotNil(t, dst.PercentageSize)
		assert.NotSame(t, src.PercentageSize, dst.PercentageSize)
		assert.NotSame(t, &src.PercentageSize.raw[0], &dst.PercentageSize.raw[0])
	})

	t.Run("zero source", func(t *testing.T) {
		t.Parallel()

		dst := MustSize("1MiB")

		require.NoError(t, dst.Merge(Size{}))

		require.NotNil(t, dst.ByteSize)
		assert.Equal(t, "1MiB", string(dst.ByteSize.raw))
	})
}
