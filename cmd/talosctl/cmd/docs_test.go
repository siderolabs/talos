// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cmd_test

import (
	"testing"

	"github.com/siderolabs/talos/cmd/talosctl/cmd"
)

func TestConvertIndentedCodeBlocks(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "single line no indent",
			input:    "hello world",
			expected: "hello world",
		},
		{
			name:     "single tab-indented line",
			input:    "Line 1\n\tcode line\nLine 3",
			expected: "Line 1\n```\ncode line\n```\nLine 3",
		},
		{
			name:     "multiple consecutive tab-indented lines",
			input:    "Intro\n\tline 1\n\tline 2\n\tline 3\nAfter",
			expected: "Intro\n```\nline 1\nline 2\nline 3\n```\nAfter",
		},
		{
			name:     "multiple groups of indented lines",
			input:    "First group:\n\tcode 1\n\tcode 2\nMiddle\n\tcode 3\n\tcode 4\nEnd",
			expected: "First group:\n```\ncode 1\ncode 2\n```\nMiddle\n```\ncode 3\ncode 4\n```\nEnd",
		},
		{
			name:     "already fenced code block untouched",
			input:    "Intro\n```\n\tcode in fence\n\tmore code\n```\nAfter",
			expected: "Intro\n```\n\tcode in fence\n\tmore code\n```\nAfter",
		},
		{
			name:     "indented lines with fenced block nearby",
			input:    "Section 1:\n\tindented\nSection 2:\n```\nfenced\n```\nSection 3:\n\tmore indented",
			expected: "Section 1:\n```\nindented\n```\nSection 2:\n```\nfenced\n```\nSection 3:\n```\nmore indented\n```",
		},
		{
			name:     "fenced block with language specifier",
			input:    "Example:\n```yaml\n\tkey: value\n```\nAfter:\n\tindented",
			expected: "Example:\n```yaml\n\tkey: value\n```\nAfter:\n```\nindented\n```",
		},
		{
			name:     "tabs at varying positions (only leading tabs count)",
			input:    "Text\n\tindent\n  spaces\nMore\n\tanother\ttab",
			expected: "Text\n```\nindent\n```\n  spaces\nMore\n```\nanother\ttab\n```",
		},
		{
			name:     "real-world cobra completion example",
			input:    "To load completions in your current shell session:\n\n\tsource <(talosctl completion bash)\n\nTo load completions for every new session:",
			expected: "To load completions in your current shell session:\n\n```\nsource <(talosctl completion bash)\n```\n\nTo load completions for every new session:",
		},
		{
			name:     "multiple fenced code blocks",
			input:    "First:\n```\ncode1\n```\nSecond:\n```\ncode2\n```\nThird:\n\tindented",
			expected: "First:\n```\ncode1\n```\nSecond:\n```\ncode2\n```\nThird:\n```\nindented\n```",
		},
		{
			name:     "toggle between fenced and indented",
			input:    "\tfenced toggle 1\n```\nin fence\n```\n\tfenced toggle 2",
			expected: "```\nfenced toggle 1\n```\n```\nin fence\n```\n```\nfenced toggle 2\n```",
		},
		{
			name:     "whitespace-only lines between indented groups",
			input:    "Start\n\tcode1\n\nMiddle\n\tcode2\nEnd",
			expected: "Start\n```\ncode1\n```\n\nMiddle\n```\ncode2\n```\nEnd",
		},
		{
			name:     "preserve line with spaces only",
			input:    "Text\n\tindented\n   \nMore",
			expected: "Text\n```\nindented\n```\n   \nMore",
		},
		{
			name:     "mixed tabs and spaces in continuation",
			input:    "Start:\n\tline with tab\n  \t  line with mixed spacing\nEnd",
			expected: "Start:\n```\nline with tab\n```\n  \t  line with mixed spacing\nEnd",
		},
		{
			name:     "tab-indented list items not wrapped",
			input:    "Items:\n\t- Item 1\n\t- Item 2\n\t- Item 3\nAfter",
			expected: "Items:\n\t- Item 1\n\t- Item 2\n\t- Item 3\nAfter",
		},
		{
			name:     "tab-indented mixed list with different markers",
			input:    "Items:\n\t- First\n\t* Second\n\t+ Third\nAfter",
			expected: "Items:\n\t- First\n\t* Second\n\t+ Third\nAfter",
		},
		{
			name:     "tab-indented non-list code wrapped",
			input:    "Code:\n\techo hello\n\techo world\nAfter",
			expected: "Code:\n```\necho hello\necho world\n```\nAfter",
		},
		{
			name:     "list with continuation paragraph",
			input:    "My list:\n\t- AAA\n\t\n\t  AAA continuation\nAfter",
			expected: "My list:\n\t- AAA\n\t\n\t  AAA continuation\nAfter",
		},
		{
			name:     "list with multiple items and continuations",
			input:    "Items:\n\t- Item 1\n\t\n\t  Continuation\n\t- Item 2\nAfter",
			expected: "Items:\n\t- Item 1\n\t\n\t  Continuation\n\t- Item 2\nAfter",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cmd.ConvertIndentedCodeBlocks(tt.input)
			if result != tt.expected {
				t.Errorf("convertIndentedCodeBlocks() mismatch\nInput:\n%q\nExpected:\n%q\nGot:\n%q", tt.input, tt.expected, result)
			}
		})
	}
}
