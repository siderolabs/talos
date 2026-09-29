// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package safeout renders untrusted text safely to a terminal.
//
// Everything talosctl prints about a node - log lines, resource fields, service
// states, hostnames, error messages - is chosen by that node. A compromised node
// can therefore put terminal control sequences into any of it, and a terminal
// acts on them: OSC 52 writes the operator's clipboard, a bare CR overwrites the
// line prefix talosctl printed itself, CSI sequences repaint the screen, and
// bidirectional overrides reorder what the operator reads. The node is the
// attacker, the operator's terminal emulator is the vulnerable interpreter, and
// the only place the two can be separated is here, on the client, at the point
// where talosctl renders.
//
// The filter is the identity transformation on printable UTF-8: well-behaved
// output passes through byte for byte, so piping talosctl into jq, grep or a
// file keeps working and only malicious or malformed content is rewritten. That
// property is what makes it safe to filter unconditionally rather than only when
// stdout is a terminal - node logs are routinely redirected to a file or captured
// by CI and rendered on a terminal much later, which is precisely when a
// terminal-detection check would have already waved the payload through.
//
// The filter deliberately does not escape a literal backslash, so an escape it
// emits is indistinguishable from that same text appearing literally in the
// source data. Preserving the identity property is worth more than that
// ambiguity: neither form can drive a terminal.
//
// Escaping is not the right answer for every kind of output, and this package is
// not meant for all of it:
//
//   - Structured output (-o json, -o yaml) is already safe, because the encoders
//     escape control characters themselves. Do not filter it again.
//   - Byte streams that are not text at all (talosctl read, pcap, copy, support
//     bundles) must reach their destination unmodified. Protect those by refusing
//     to write raw bytes to a terminal, not by escaping them.
//   - Escape sequences talosctl emits itself - colors, spinners, the dashboard -
//     must be written outside this filter, otherwise talosctl escapes its own
//     styling. Filter the untrusted field with [String] before interpolating it
//     into a line that is styled later.
package safeout

import (
	"fmt"
	"io"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

// RawOutputEnvVar disables filtering of the stdout and stderr streams when set
// to a true value, for an operator who needs the original bytes from a node
// (a service which legitimately colorizes its own log output, say).
//
// It does not disable [String] and [Cell]: a field interpolated into a line
// talosctl formats and colorizes itself is escaped either way, as leaving that
// one field raw is the injection this package exists to prevent.
const RawOutputEnvVar = "TALOSCTL_RAW_OUTPUT"

// rawOutput reports whether the stdout and stderr streams should pass through
// unfiltered.
var rawOutput = sync.OnceValue(func() bool {
	raw, err := strconv.ParseBool(os.Getenv(RawOutputEnvVar))

	return err == nil && raw
})

// allowed reports whether r may reach a terminal as itself.
//
// unicode.IsPrint is false for exactly the runes which are dangerous or invisible
// here: the C0 controls including ESC, BEL, CR and DEL, the C1 controls including
// the single-byte CSI U+009B, the bidirectional overrides and isolates
// U+202A-U+202E and U+2066-U+2069, the zero-width formatting characters, the byte
// order mark, and the line and paragraph separators U+2028 and U+2029.
//
// Explicitly allow `\n` and `\t` as they are used in table formatting.
func allowed(r rune) bool {
	switch r {
	case '\n', '\t':
		return true
	default:
		return unicode.IsPrint(r)
	}
}

const hexDigits = "0123456789abcdef"

// appendEscapedByte appends a byte which is not part of a valid UTF-8 sequence.
func appendEscapedByte(dst []byte, b byte) []byte {
	return append(dst, '\\', 'x', hexDigits[b>>4], hexDigits[b&0xf])
}

// appendEscapedRune appends the visible representation of a disallowed rune.
//
// The escapes are uniform rather than using the short Go forms such as \r, so
// that everything this package rewrites can be found with a single search for
// \x or \u, and so that no rewritten rune can be mistaken for one which was
// allowed through.
func appendEscapedRune(dst []byte, r rune) []byte {
	switch {
	case r < 0x100:
		return appendEscapedByte(dst, byte(r))
	case r < 0x10000:
		dst = append(dst, '\\', 'u')

		for shift := 12; shift >= 0; shift -= 4 {
			dst = append(dst, hexDigits[(r>>shift)&0xf])
		}

		return dst
	default:
		dst = append(dst, '\\', 'U')

		for shift := 28; shift >= 0; shift -= 4 {
			dst = append(dst, hexDigits[(r>>shift)&0xf])
		}

		return dst
	}
}

// escape appends the filtered form of src to dst.
//
// It returns the number of trailing bytes of src which were not consumed because
// they are the start of a UTF-8 sequence which src is too short to complete. A
// streaming caller holds those back for the next chunk; a caller with the whole
// string in hand escapes them as individual bytes.
func escape(dst, src []byte, allowed func(rune) bool) ([]byte, int) {
	for len(src) > 0 {
		r, size := utf8.DecodeRune(src)

		// DecodeRune reports (RuneError, 1) both for a byte which can never be part
		// of a valid sequence and for a valid prefix which is cut short; a genuine
		// U+FFFD in the input decodes with size 3 and is not confused with either.
		if r == utf8.RuneError && size <= 1 {
			if !utf8.FullRune(src) {
				return dst, len(src)
			}

			dst = appendEscapedByte(dst, src[0])
			src = src[1:]

			continue
		}

		if allowed(r) {
			dst = append(dst, src[:size]...)
		} else {
			dst = appendEscapedRune(dst, r)
		}

		src = src[size:]
	}

	return dst, 0
}

// Writer filters everything written through it before passing it on.
//
// It is safe for concurrent use, and it is stream oriented: log output arrives
// from the API in arbitrary chunks, so a multi-byte rune split across two Write
// calls is held back until the following call completes it rather than being
// mangled into escapes. [Writer.Flush] releases such a remainder when no further
// call is coming.
type Writer struct {
	w io.Writer

	mu sync.Mutex
	// pending holds an incomplete trailing UTF-8 sequence, at most utf8.UTFMax-1 bytes.
	pending []byte
	joined  []byte
	out     []byte

	raw bool
}

// NewWriter returns a Writer filtering everything written to w.
func NewWriter(w io.Writer) *Writer {
	return &Writer{w: w}
}

// Write implements io.Writer.
func (w *Writer) Write(p []byte) (int, error) {
	if w.raw {
		return w.w.Write(p)
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	src := p

	if len(w.pending) > 0 {
		w.joined = append(append(w.joined[:0], w.pending...), p...)
		src = w.joined
		w.pending = w.pending[:0]
	}

	out, held := escape(w.out[:0], src, allowed)
	w.out = out
	w.pending = append(w.pending[:0], src[len(src)-held:]...)

	if _, err := w.w.Write(out); err != nil {
		return 0, err
	}

	// report the caller's length: the filtered form has a different one, and a
	// short write would make fmt and io.Copy believe the write failed.
	return len(p), nil
}

// Flush escapes and writes out an incomplete UTF-8 sequence held back by the
// last Write. It must be called before the process exits, or a truncated
// trailing rune is silently dropped.
func (w *Writer) Flush() error {
	if w.raw {
		return nil
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	if len(w.pending) == 0 {
		return nil
	}

	out := w.out[:0]

	for _, b := range w.pending {
		out = appendEscapedByte(out, b)
	}

	w.pending = w.pending[:0]
	w.out = out

	_, err := w.w.Write(out)

	return err
}

var (
	stdout = sync.OnceValue(func() *Writer { return &Writer{w: os.Stdout, raw: rawOutput()} })
	stderr = sync.OnceValue(func() *Writer { return &Writer{w: os.Stderr, raw: rawOutput()} })
)

// Stdout returns the filtered standard output stream.
//
// Anything rendering node-supplied text for a human writes here instead of to
// os.Stdout, including as the writer underneath a tabwriter.
func Stdout() io.Writer { return stdout() }

// Stderr returns the filtered standard error stream.
func Stderr() io.Writer { return stderr() }

// Fprintf is fmt.Fprintf with every string and error argument escaped as a table
// cell by [Cell].
//
// Use it for a row handed to a tabwriter. The format string stays untouched
// because it is talosctl's own literal, so the tabs which separate the columns
// keep their meaning; the values interpolated into it come from the node, and a
// tab in one of those is read by the tabwriter as a column separator before any
// writer underneath it can see the byte.
func Fprintf(w io.Writer, format string, a ...any) (int, error) {
	escaped := a

	for i, v := range a {
		var cell string

		switch v := v.(type) {
		case string:
			cell = Cell(v)
		case error:
			// fmt renders an error whose Error method panics on a nil receiver as
			// <nil>, so the rendering goes through fmt here too rather than calling
			// the method directly.
			cell = Cell(fmt.Sprint(v))
		default:
			continue
		}

		// copy on first write: a caller passing a slice with `args...` keeps
		// ownership of it.
		if &escaped[0] == &a[0] {
			escaped = slices.Clone(a)
		}

		escaped[i] = cell
	}

	return fmt.Fprintf(w, format, escaped...)
}

// Print is fmt.Print against the filtered standard output stream.
func Print(a ...any) (int, error) { return fmt.Fprint(Stdout(), a...) }

// Printf is fmt.Printf against the filtered standard output stream.
func Printf(format string, a ...any) (int, error) { return fmt.Fprintf(Stdout(), format, a...) }

// Println is fmt.Println against the filtered standard output stream.
func Println(a ...any) (int, error) { return fmt.Fprintln(Stdout(), a...) }

// Warningf prints a warning to the filtered standard error stream, escaping every
// string and error argument as a table cell by [Cell].
//
// It replaces cli.Warning for the commands which talk to a node: a warning is a
// single line prefixed with WARNING:, and its text is usually chosen by the node
// - a config validation warning, a partially failed request - so a newline in it
// would let the node write a line of its own without that prefix.
func Warningf(format string, a ...any) {
	if !strings.HasSuffix(format, "\n") {
		format += "\n"
	}

	Fprintf(Stderr(), "WARNING: "+format, a...) //nolint:errcheck
}

// Flush releases any partial rune held by the shared streams.
func Flush() error {
	if err := stdout().Flush(); err != nil {
		return err
	}

	return stderr().Flush()
}

// String returns s with anything a terminal would act on replaced by a visible
// escape, preserving newlines and tabs.
//
// Use it for an untrusted field which is interpolated into a line that something
// other than [Stdout] renders - a reporter message, a colorized status line - so
// that talosctl's own escape sequences survive while the node's do not.
func String(s string) string {
	return escapeString(s, allowed)
}

// Cell returns s escaped for a single field of a table or a one-line summary,
// additionally escaping newlines and tabs.
//
// A tab in a value handed to a tabwriter is read as a column separator and a
// newline ends the row, so a node choosing either one silently rewrites the shape
// of the table around it.
func Cell(s string) string {
	return escapeString(s, unicode.IsPrint)
}

func escapeString(s string, allowed func(rune) bool) string {
	// ranging over a string yields utf8.RuneError for a byte which is not part of
	// a valid sequence, so this catches malformed input as well as disallowed
	// runes. A genuine U+FFFD sends a well-formed string down the slow path, where
	// it is allowed through unchanged.
	clean := true

	for _, r := range s {
		if r == utf8.RuneError || !allowed(r) {
			clean = false

			break
		}
	}

	if clean {
		return s
	}

	src := []byte(s)

	out, held := escape(make([]byte, 0, len(s)+escapeGrowth), src, allowed)

	// nothing further is coming, so a truncated trailing sequence is escaped
	// rather than held back for a chunk which will never arrive.
	for _, b := range src[len(src)-held:] {
		out = appendEscapedByte(out, b)
	}

	return string(out)
}

// escapeGrowth is the slack left in the escaped buffer, enough for a couple of
// escapes before it has to grow.
const escapeGrowth = 16
