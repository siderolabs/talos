// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//nolint:testpackage
package components

import (
	"testing"
	"unicode/utf8"

	"github.com/rivo/tview"
)

// TestFormatLogEntry covers the two responsibilities of formatLogEntry: deciding
// whether an entry is visible under the current filter, and rendering it as tview
// markup with the filter matches highlighted.
//
// The multi-byte cases exist because lowercasing can change the byte length of a
// string (Go applies simple case mapping, so U+212A KELVIN SIGN and U+0130 LATIN
// CAPITAL LETTER I WITH DOT ABOVE both shrink). A byte index taken from the
// lowercased text is therefore not a valid index into the original text, and using
// one either splits a rune or runs past the end of the string.
func TestFormatLogEntry(t *testing.T) {
	for _, test := range []struct {
		name     string
		entry    logEntry
		filter   string
		expected string
		expectOk bool
	}{
		{
			// An empty filter shows every entry verbatim, with a trailing newline.
			name:     "no filter",
			entry:    logEntry{text: "hello world"},
			expected: "hello world\n",
			expectOk: true,
		},
		{
			// Error entries are wrapped in the red color tag, as they were before
			// filtering existed.
			name:     "no filter, error entry",
			entry:    logEntry{text: "boom", isError: true},
			expected: "[red]boom[-]\n",
			expectOk: true,
		},
		{
			// Log lines are not trusted as markup: a literal "[red]" in the log must
			// be escaped so tview renders it instead of switching color.
			name:     "no filter escapes markup",
			entry:    logEntry{text: "[red]not a color"},
			expected: "[red[]not a color\n",
			expectOk: true,
		},
		{
			// A non-matching entry is hidden: ok is false and the caller writes nothing.
			name:     "filter does not match",
			entry:    logEntry{text: "hello world"},
			filter:   "nope",
			expected: "",
			expectOk: false,
		},
		{
			// Matching is case-insensitive, but the highlighted span is taken from the
			// original text, so the log line keeps its own casing.
			name:     "filter matches, original casing preserved",
			entry:    logEntry{text: "Hello World"},
			filter:   "hello",
			expected: "[yellow]Hello[-] World\n",
			expectOk: true,
		},
		{
			// The same, with the casing difference on the filter side.
			name:     "uppercase filter matches lowercase text",
			entry:    logEntry{text: "hello world"},
			filter:   "WORLD",
			expected: "hello [yellow]world[-]\n",
			expectOk: true,
		},
		{
			// Every occurrence is highlighted, not only the first, and the search
			// resumes after each match rather than rescanning it.
			name:     "multiple matches",
			entry:    logEntry{text: "ab AB ab"},
			filter:   "ab",
			expected: "[yellow]ab[-] [yellow]AB[-] [yellow]ab[-]\n",
			expectOk: true,
		},
		{
			// Closing a highlight with "[-]" resets to the default color, not to red,
			// so the red tag has to be re-emitted after every match on an error entry.
			name:     "error entry restores base color after each match",
			entry:    logEntry{text: "err x err", isError: true},
			filter:   "err",
			expected: "[red][yellow]err[-][red] x [yellow]err[-][red][-]\n",
			expectOk: true,
		},
		{
			// Escaping still applies to the segments around and inside a highlight.
			name:     "match escapes markup",
			entry:    logEntry{text: "[red]hi"},
			filter:   "hi",
			expected: "[red[][yellow]hi[-]\n",
			expectOk: true,
		},
		{
			// The filter is a 3-byte U+212A KELVIN SIGN that lowercases to a 1-byte
			// "k". Widening the match by len(filter) instead of by the lowercased
			// length reaches two bytes past the end of a 4-byte line, which used to
			// panic with "slice bounds out of range".
			name:     "filter lowercases to fewer bytes",
			entry:    logEntry{text: "abck"},
			filter:   "K",
			expected: "abc[yellow]k[-]\n",
			expectOk: true,
		},
		{
			// Same shrinkage, this time in the log line: the KELVIN SIGN ahead of the
			// match makes the lowercased text two bytes shorter than the original, so
			// a lowercased index points into the middle of that rune.
			name:     "text lowercases to fewer bytes",
			entry:    logEntry{text: "Kx"},
			filter:   "x",
			expected: "K[yellow]x[-]\n",
			expectOk: true,
		},
		{
			// The match lands on the rune that changes length, so the highlighted span
			// is 3 bytes wide in the original while the filter that found it is 1.
			name:     "match on the shrinking rune itself",
			entry:    logEntry{text: "aKb"},
			filter:   "k",
			expected: "a[yellow]K[-]b\n",
			expectOk: true,
		},
		{
			// A second, more realistic shrinking rune: U+0130 LATIN CAPITAL LETTER I
			// WITH DOT ABOVE is 2 bytes and lowercases to a 1-byte "i". The whole line
			// is one match, so the shrinkage falls inside the highlighted span.
			name:     "turkish dotted capital i",
			entry:    logEntry{text: "İstanbul"},
			filter:   "istanbul",
			expected: "[yellow]İstanbul[-]\n",
			expectOk: true,
		},
		{
			// Log lines come off the wire and are not guaranteed to be valid UTF-8.
			// Ranging over such a string yields RuneError for the bad byte, which
			// still has to produce a usable offset instead of panicking.
			name:     "invalid utf-8 does not panic",
			entry:    logEntry{text: "a\xffb"},
			filter:   "b",
			expected: "a\xff[yellow]b[-]\n",
			expectOk: true,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			actual, ok := formatLogEntry(test.entry, test.filter)

			if ok != test.expectOk {
				t.Fatalf("ok = %v, expected %v", ok, test.expectOk)
			}

			if actual != test.expected {
				t.Fatalf("got %q, expected %q", actual, test.expected)
			}
		})
	}
}

// TestFormatLogEntryKeepsTextValid checks the corruption half of the byte-index
// bug, which the table above cannot express as an expected string: with enough
// shrinkage ahead of the match, a lowercased index lands inside a multi-byte rune
// and the rendered line silently becomes invalid UTF-8 without ever panicking.
func TestFormatLogEntryKeepsTextValid(t *testing.T) {
	// Two KELVIN SIGNs put the lowercased text four bytes behind the original, so
	// the old code split the second one and emitted a lone continuation byte.
	actual, ok := formatLogEntry(logEntry{text: "KKx"}, "x")
	if !ok {
		t.Fatal("expected the entry to match")
	}

	if !utf8.ValidString(actual) {
		t.Fatalf("output is not valid UTF-8: %q", actual)
	}
}

// TestDeactivateSearchKeepFilter verifies that Enter (clearText=false) hides the
// input row while preserving the filter text for next activation.
func TestDeactivateSearchKeepFilter(t *testing.T) {
	app := tview.NewApplication()
	defer app.Stop()

	viewer := NewLogViewer(app)

	// Manually set up the filtered state as activateSearch would.
	viewer.filterText = "test"
	viewer.filterActive = true
	viewer.filterInput.SetText("test")
	viewer.SetRows(1, 0, 1)
	viewer.AddItem(viewer.filterInput, 2, 0, 1, 1, 0, 0, true)

	// Simulate Enter: keep the filter but hide the input row.
	viewer.deactivateSearch(false)

	if viewer.filterActive {
		t.Fatalf("filterActive = %v, expected false (input row should be hidden)", viewer.filterActive)
	}

	if viewer.filterText != "test" {
		t.Fatalf("filterText = %q, expected %q (filter text should be preserved for next /)", viewer.filterText, "test")
	}
}

// TestDeactivateSearchClearFilter verifies that Escape (clearText=true) clears
// the filter and hides the input row.
func TestDeactivateSearchClearFilter(t *testing.T) {
	app := tview.NewApplication()
	defer app.Stop()

	viewer := NewLogViewer(app)

	// Manually set up the filtered state.
	viewer.filterText = "test"
	viewer.filterActive = true
	viewer.filterInput.SetText("test")
	viewer.SetRows(1, 0, 1)
	viewer.AddItem(viewer.filterInput, 2, 0, 1, 1, 0, 0, true)

	// Add a log entry so renderLogs has something to process.
	viewer.entries = append(viewer.entries, logEntry{text: "test line", isError: false})

	// Simulate Escape: clear the filter and hide the input row.
	viewer.deactivateSearch(true)

	if viewer.filterActive {
		t.Fatalf("filterActive = %v, expected false", viewer.filterActive)
	}

	if viewer.filterText != "" {
		t.Fatalf("filterText = %q, expected empty", viewer.filterText)
	}

	if viewer.filterInput.GetText() != "" {
		t.Fatalf("filterInput text = %q, expected empty", viewer.filterInput.GetText())
	}
}

// TestDeactivateSearchWhenInactive verifies that calling deactivateSearch when
// the input is not active clears the text only if clearText=true.
func TestDeactivateSearchWhenInactive(t *testing.T) {
	app := tview.NewApplication()
	defer app.Stop()

	viewer := NewLogViewer(app)
	viewer.filterText = "old"
	viewer.filterActive = false

	// clearText=false: should not clear.
	viewer.deactivateSearch(false)

	if viewer.filterText != "old" {
		t.Fatalf("filterText = %q, expected %q", viewer.filterText, "old")
	}

	// clearText=true: should clear and re-render.
	viewer.entries = append(viewer.entries, logEntry{text: "line", isError: false})
	viewer.deactivateSearch(true)

	if viewer.filterText != "" {
		t.Fatalf("filterText = %q, expected empty", viewer.filterText)
	}
}

// TestDeactivateSearchEscapeWithoutFilterKeepsNoData verifies that pressing Esc
// immediately after opening the filter (without typing) does not clear the
// initial noData placeholder.
func TestDeactivateSearchEscapeWithoutFilterKeepsNoData(t *testing.T) {
	app := tview.NewApplication()
	defer app.Stop()

	viewer := NewLogViewer(app)
	// Manually activate the filter input (as activateSearch would).
	viewer.filterActive = true
	viewer.SetRows(1, 0, 1)
	viewer.AddItem(viewer.filterInput, 2, 0, 1, 1, 0, 0, true)
	viewer.deactivateSearch(true)

	if text := viewer.logs.GetText(true); text != noData {
		t.Fatalf("logs text = %q, expected %q", text, noData)
	}
}

// TestLowerWithOffsets checks the invariants formatLogEntry relies on when it maps
// a position found in the lowercased text back onto the original: the offsets cover
// every byte of the lowered result plus one trailing sentinel, they never decrease,
// and they always stay within the original string. Together those guarantee that
// slicing the original through them can neither run out of bounds nor split a rune.
func TestLowerWithOffsets(t *testing.T) {
	for _, test := range []struct {
		name     string
		text     string
		expected string
	}{
		// The empty string still gets the sentinel; ASCII exercises the one-byte-per-
		// offset path; Cyrillic keeps the byte length across the case change; the last
		// case shrinks by two bytes on the first rune and one on the second.
		{name: "empty", text: "", expected: ""},
		{name: "ascii", text: "AbC", expected: "abc"},
		{name: "multi-byte, same length", text: "ЖУК", expected: "жук"},
		{name: "multi-byte, shrinking", text: "Kİ", expected: "ki"},
	} {
		t.Run(test.name, func(t *testing.T) {
			lowered, offsets := lowerWithOffsets(test.text)

			if lowered != test.expected {
				t.Fatalf("got %q, expected %q", lowered, test.expected)
			}

			if len(offsets) != len(lowered)+1 {
				t.Fatalf("len(offsets) = %d, expected %d", len(offsets), len(lowered)+1)
			}

			if offsets[len(offsets)-1] != len(test.text) {
				t.Fatalf("sentinel = %d, expected %d", offsets[len(offsets)-1], len(test.text))
			}

			previous := 0

			for i, offset := range offsets {
				if offset < previous {
					t.Fatalf("offsets[%d] = %d is smaller than the previous offset %d", i, offset, previous)
				}

				if offset > len(test.text) {
					t.Fatalf("offsets[%d] = %d is out of range for a %d-byte string", i, offset, len(test.text))
				}

				previous = offset
			}
		})
	}
}
