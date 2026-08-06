// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package output_test

import (
	"errors"
	"strings"
	"testing"
	"testing/iotest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/internal/app/lifecycle/internal/output"
)

func TestStream(t *testing.T) {
	longLine := strings.Repeat("x", 1024) + "\n"
	input := "machine configuration is invalid: café\n" + longLine + "partial line"

	var messages []string

	err := output.Stream(iotest.OneByteReader(strings.NewReader(input)), func(message string) error {
		messages = append(messages, message)

		return nil
	})
	require.NoError(t, err)

	assert.Equal(t, []string{
		"machine configuration is invalid: café\n",
		longLine,
		"partial line",
	}, messages)
}

func TestStreamErrors(t *testing.T) {
	t.Run("oversized line", func(t *testing.T) {
		var messages []string

		err := output.Stream(strings.NewReader(strings.Repeat("x", 2*1024*1024)), func(message string) error {
			messages = append(messages, message)

			return nil
		})

		require.NoError(t, err)
		require.Len(t, messages, 2)
		assert.Len(t, messages[0], 1024*1024)
		assert.Len(t, messages[1], 1024*1024)
	})

	t.Run("reader", func(t *testing.T) {
		err := output.Stream(iotest.ErrReader(errors.New("read failed")), func(string) error {
			return nil
		})

		require.Error(t, err)
		assert.ErrorContains(t, err, "read failed")
	})

	t.Run("sender", func(t *testing.T) {
		reader := strings.NewReader("line\nremaining output\n")

		err := output.Stream(reader, func(string) error {
			return errors.New("send failed")
		})

		require.Error(t, err)
		assert.ErrorContains(t, err, "send failed")
		assert.Zero(t, reader.Len(), "output should be drained after a sender failure")
	})
}
