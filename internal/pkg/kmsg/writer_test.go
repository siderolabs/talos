// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kmsg_test

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/talos-systems/talos/internal/pkg/kmsg"
)

type fakeWriter struct {
	lines [][]byte
}

func (w *fakeWriter) Write(p []byte) (n int, err error) {
	w.lines = append(w.lines, append([]byte(nil), p...))

	return len(p), nil
}

func TestWriter(t *testing.T) {
	fakeW := &fakeWriter{}
	kmsgW := &kmsg.Writer{KmsgWriter: fakeW}

	n, err := kmsgW.Write([]byte("foo"))
	assert.Equal(t, 3, n)
	assert.NoError(t, err)

	n, err = kmsgW.Write([]byte("bar\n"))
	assert.Equal(t, 4, n)
	assert.NoError(t, err)

	n, err = kmsgW.Write([]byte("foo\nbar\n"))
	assert.Equal(t, 8, n)
	assert.NoError(t, err)

	n, err = kmsgW.Write(append(bytes.Repeat([]byte{0xce}, kmsg.MaxLineLength-1), '\n'))
	assert.Equal(t, kmsg.MaxLineLength, n)
	assert.NoError(t, err)

	n, err = kmsgW.Write(append(bytes.Repeat([]byte{0xce}, kmsg.MaxLineLength), '\n', 'a', 'b', '\n'))
	assert.Equal(t, kmsg.MaxLineLength+4, n)
	assert.NoError(t, err)

	assert.Len(t, fakeW.lines, 7)
	assert.Equal(t, fakeW.lines[0], []byte("foo"))
	assert.Equal(t, fakeW.lines[1], []byte("bar\n"))
	assert.Equal(t, fakeW.lines[2], []byte("foo\n"))
	assert.Equal(t, fakeW.lines[3], []byte("bar\n"))
	assert.Equal(t, fakeW.lines[4], append(bytes.Repeat([]byte{0xce}, kmsg.MaxLineLength-1), '\n'))
	assert.Equal(t, fakeW.lines[5], append(bytes.Repeat([]byte{0xce}, kmsg.MaxLineLength-4), '.', '.', '.', '\n'))
	assert.Equal(t, fakeW.lines[6], []byte("ab\n"))
}
