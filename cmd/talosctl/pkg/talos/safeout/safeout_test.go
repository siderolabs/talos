// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package safeout_test

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"unicode"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/cmd/talosctl/pkg/talos/safeout"
)

// payloads that can put into any string talosctl renders.
var payloads = map[string]struct {
	in       string
	expected string
}{
	"OSC 0 window title": {
		in:       "\x1b]0;F3-OSC-TITLE-INJECTED\x07",
		expected: `\x1b]0;F3-OSC-TITLE-INJECTED\x07`,
	},
	"OSC 52 clipboard write": {
		in:       "\x1b]52;c;RjMtQ0xJUEJPQVJELUhJSkFDSw==\x07",
		expected: `\x1b]52;c;RjMtQ0xJUEJPQVJELUhJSkFDSw==\x07`,
	},
	"CSI screen repaint": {
		in:       "\x1b[2J\x1b[HF3-SCREEN-REPAINTED-BY-NODE",
		expected: `\x1b[2J\x1b[HF3-SCREEN-REPAINTED-BY-NODE`,
	},
	"bare CR overwriting the line prefix": {
		in:       "junk\rF3-LINE-PREFIX-OVERWRITTEN",
		expected: `junk\x0dF3-LINE-PREFIX-OVERWRITTEN`,
	},
	"SGR color in a hostname": {
		in:       "\x1b[1;31mF3-HOSTNAME-IS-A-SINK\x1b[0m",
		expected: `\x1b[1;31mF3-HOSTNAME-IS-A-SINK\x1b[0m`,
	},
	// U+009B is a single byte CSI on a terminal which decodes C1, and arrives as
	// a two byte UTF-8 sequence rather than as ESC.
	"C1 single byte CSI": {
		in:       "before\u009b2Jcsi",
		expected: `before\x9b2Jcsi`,
	},
	"bidirectional override": {
		in:       "safe\u202eesrever\u202c",
		expected: `safe\u202eesrever\u202c`,
	},
	"bidirectional isolate": {
		in:       "a\u2066b\u2069c",
		expected: `a\u2066b\u2069c`,
	},
	"zero width space": {
		in:       "ad\u200bmin",
		expected: `ad\u200bmin`,
	},
	"byte order mark": {
		in:       "\ufeffvalue",
		expected: `\ufeffvalue`,
	},
	"line and paragraph separators": {
		in:       "a\u2028b\u2029c",
		expected: `a\u2028b\u2029c`,
	},
	"DEL and backspace": {
		in:       "pass\x7fword\x08\x08",
		expected: `pass\x7fword\x08\x08`,
	},
	"BEL": {
		in:       "beep\a",
		expected: `beep\x07`,
	},
	"vertical tab and form feed": {
		in:       "a\vb\fc",
		expected: `a\x0bb\x0cc`,
	},
	"NUL": {
		in:       "a\x00b",
		expected: `a\x00b`,
	},
	"invalid UTF-8": {
		in:       "bad\xffbyte\xc3",
		expected: `bad\xffbyte\xc3`,
	},
	"surrogate half encoded as UTF-8": {
		in:       "a\xed\xa0\x80b",
		expected: `a\xed\xa0\x80b`,
	},
	"unassigned plane 15 code point": {
		in:       "a\U000f0000b",
		expected: `a\U000f0000b`,
	},
}

// content which must survive untouched: the filter is the identity
// transformation on printable UTF-8.
var passthrough = []string{
	"",
	"kubelet: starting",
	"[   12.345678] kernel: nvme nvme0: pci function",
	`{"ts":1234,"msg":"hello","level":"info"}`,
	"column\tseparated\tvalues\n",
	"multi\nline\noutput\n",
	"ünïcode ÿ ß 日本語 emoji \U0001f389 combining é", //nolint:gosmopolitan // intentionally testing non-ASCII UTF-8
	`path/with\backslash and \x1b written literally`,
	"replacement \ufffd character",
}

func TestStringEscapes(t *testing.T) {
	t.Parallel()

	for name, tc := range payloads {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, tc.expected, safeout.String(tc.in))
		})
	}
}

func TestStringPassthrough(t *testing.T) {
	t.Parallel()

	for _, in := range passthrough {
		assert.Equal(t, in, safeout.String(in), "printable UTF-8 must pass through unchanged")
	}
}

func TestWriterMatchesString(t *testing.T) {
	t.Parallel()

	for name, tc := range payloads {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			var buf bytes.Buffer

			w := safeout.NewWriter(&buf)

			n, err := w.Write([]byte(tc.in))
			require.NoError(t, err)
			// a filtering writer changes the byte count, but must report the caller's
			// length or fmt and io.Copy treat the write as short and failed.
			assert.Equal(t, len(tc.in), n)

			require.NoError(t, w.Flush())

			assert.Equal(t, tc.expected, buf.String())
		})
	}
}

func TestWriterPassthrough(t *testing.T) {
	t.Parallel()

	for _, in := range passthrough {
		var buf bytes.Buffer

		w := safeout.NewWriter(&buf)

		_, err := w.Write([]byte(in))
		require.NoError(t, err)
		require.NoError(t, w.Flush())

		assert.Equal(t, in, buf.String())
	}
}

// TestWriterSplitRune covers the case a stream filter gets wrong: node output
// arrives in arbitrary chunks, so a multi-byte rune is routinely cut in half by
// the transport. Every split of every input must produce the same bytes as
// writing it in one go.
func TestWriterSplitRune(t *testing.T) {
	t.Parallel()

	inputs := append([]string{}, passthrough...)

	for _, tc := range payloads {
		inputs = append(inputs, tc.in)
	}

	for _, in := range inputs {
		for split := range len(in) + 1 {
			var buf bytes.Buffer

			w := safeout.NewWriter(&buf)

			_, err := w.Write([]byte(in[:split]))
			require.NoError(t, err)

			_, err = w.Write([]byte(in[split:]))
			require.NoError(t, err)

			require.NoError(t, w.Flush())

			assert.Equal(t, safeout.String(in), buf.String(), "input %q split at %d", in, split)
		}
	}
}

// TestWriterByteAtATime is the degenerate case of the above: every rune split at
// every boundary at once.
func TestWriterByteAtATime(t *testing.T) {
	t.Parallel()

	for _, in := range passthrough {
		var buf bytes.Buffer

		w := safeout.NewWriter(&buf)

		for i := range len(in) {
			_, err := w.Write([]byte(in[i : i+1]))
			require.NoError(t, err)
		}

		require.NoError(t, w.Flush())

		assert.Equal(t, in, buf.String())
	}
}

// TestFlushReleasesTruncatedRune: a stream ending mid-rune must still render the
// bytes it did receive, rather than dropping them.
func TestFlushReleasesTruncatedRune(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	w := safeout.NewWriter(&buf)

	_, err := w.Write([]byte("ok\xe6\x97"))
	require.NoError(t, err)

	assert.Equal(t, "ok", buf.String(), "an incomplete rune is held back, not guessed at")

	require.NoError(t, w.Flush())

	assert.Equal(t, `ok\xe6\x97`, buf.String())
}

func TestCellEscapesTabAndNewline(t *testing.T) {
	t.Parallel()

	// a tab is a column separator to tabwriter and a newline ends the row, so a
	// node choosing either rewrites the shape of the table around it.
	assert.Equal(t, `a\x09b\x0ac`, safeout.Cell("a\tb\nc"))
	assert.Equal(t, "a\tb\nc", safeout.String("a\tb\nc"))
	assert.Equal(t, "plain value", safeout.Cell("plain value"))
	assert.Equal(t, `\x1b[2Jwiped`, safeout.Cell("\x1b[2Jwiped"))
}

// TestNoControlRunesSurvive asserts the property the advisory is about directly,
// rather than through golden strings.
func TestNoControlRunesSurvive(t *testing.T) {
	t.Parallel()

	var all strings.Builder

	for _, tc := range payloads {
		all.WriteString(tc.in)
	}

	for _, out := range []string{safeout.String(all.String()), safeout.Cell(all.String())} {
		for _, r := range out {
			assert.True(t, r == '\n' || r == '\t' || unicode.IsPrint(r), "rune %U reached the terminal", r)
		}
	}
}

type nilError struct {
	msg string
}

func (e *nilError) Error() string { return e.msg }

func TestFprintfEscapesArguments(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer

	args := []any{"a\tb", errors.New("\x1b[2Jwiped"), 42}

	_, err := safeout.Fprintf(&buf, "%s|%s|%d\n", args...)
	require.NoError(t, err)

	assert.Equal(t, `a\x09b|\x1b[2Jwiped|42`+"\n", buf.String())
	// the caller keeps ownership of a slice passed with args...
	assert.Equal(t, "a\tb", args[0])
}

func TestFprintfTypedNilError(t *testing.T) {
	t.Parallel()

	var (
		buf bytes.Buffer
		e   *nilError
	)

	// a nil pointer with an Error method whose receiver it dereferences: fmt
	// renders it as <nil>, and neither may this panic.
	_, err := safeout.Fprintf(&buf, "%s\n", error(e))
	require.NoError(t, err)

	assert.Equal(t, "<nil>\n", buf.String())
}
