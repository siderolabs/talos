// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package tail_test

import (
	"bufio"
	"bytes"
	"io"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/talos-systems/talos/pkg/tail"
)

func TestSkipLines(t *testing.T) {
	for _, test := range []struct {
		desc        string
		input       []byte
		tailLines   []int
		expectLines []int
	}{
		{
			desc:        "empty",
			input:       nil,
			tailLines:   []int{0, 1, 2, 10},
			expectLines: []int{0, 0, 0, 0},
		},
		{
			desc:        "enormous line",
			input:       bytes.Repeat([]byte{0xde, 0xad, 0xbe, 0xef}, 8000),
			tailLines:   []int{0, 1, 10},
			expectLines: []int{0, 1, 1},
		},
		{
			desc:        "enormous line with \\n",
			input:       append(bytes.Repeat([]byte{0xde, 0xad, 0xbe, 0xef}, 8000), '\n'),
			tailLines:   []int{0, 1, 10},
			expectLines: []int{0, 1, 1},
		},
		{
			desc:        "many small lines",
			input:       bytes.Repeat([]byte{0xde, 0xad, 0xbe, 0xef, '\n'}, 1024),
			tailLines:   []int{0, 1, 3, 10, 100, 1000},
			expectLines: []int{0, 1, 3, 10, 100, 1000},
		},
		{
			desc:        "many small aligned lines",
			input:       bytes.Repeat([]byte{0xde, 0xad, 0xbe, '\n'}, 1024),
			tailLines:   []int{0, 1, 3, 10, 100, 1000},
			expectLines: []int{0, 1, 3, 10, 100, 1000},
		},
		{
			desc:        "empty lines",
			input:       bytes.Repeat([]byte{'\n'}, 65536),
			tailLines:   []int{0, 1, 3, 10, 100, 1000, 10000},
			expectLines: []int{0, 1, 3, 10, 100, 1000, 10000},
		},
		{
			desc:        "window-sized lines",
			input:       bytes.Repeat(append(bytes.Repeat([]byte{'a'}, tail.Window-1), '\n'), 24),
			tailLines:   []int{0, 1, 3, 10, 100, 1000},
			expectLines: []int{0, 1, 3, 10, 24, 24},
		},
		{
			desc:        "long lines",
			input:       bytes.Repeat(append(bytes.Repeat([]byte{'a'}, 356), '\n'), 24),
			tailLines:   []int{0, 1, 3, 10, 15, 24, 100, 1000},
			expectLines: []int{0, 1, 3, 10, 15, 24, 24, 24},
		},
	} {
		for i, lines := range test.tailLines {
			r := bytes.NewReader(test.input)

			err := tail.SeekLines(r, lines)
			assert.NoError(t, err, "test %q", test.desc)

			tailOffset, _ := r.Seek(0, io.SeekCurrent) //nolint:errcheck

			expected := test.expectLines[i]
			actual := 0

			scanner := bufio.NewScanner(r)

			for scanner.Scan() {
				actual++
			}

			assert.NoError(t, scanner.Err(), "test %q", test.desc)

			assert.Equal(t, expected, actual, "test %q, tailOffset %d", test.desc, tailOffset)
		}
	}
}
