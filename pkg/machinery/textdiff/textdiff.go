// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package textdiff provides a way to compare two text blobs.
package textdiff

import (
	"fmt"
	"strings"

	"github.com/neticdk/go-stdlib/diff/myers"
)

// MaxLines is the maximum number of lines that the diff function will process before giving up and returning a message instead.
const MaxLines = 75_000

// Diff is a function that computes unified diff between two strings.
//
// The diff is limited to MaxLines lines, and if the diff is larger than that, a message is returned instead of the actual diff.
// This is to prevent the function from consuming too much memory or CPU time when comparing large text blobs.
func Diff(a, b string) (string, error) {
	if a == b {
		return "", nil
	}

	prevLines := strings.Count(a, "\n")
	newLines := strings.Count(b, "\n")

	if prevLines+newLines > MaxLines {
		return fmt.Sprintf("@@ -%d,%d +%d,%d @@ diff too large to display\n", 1, prevLines, 1, newLines), nil
	}

	differ := myers.NewCustomDiffer(
		myers.WithUnifiedFormatter(),
		myers.WithLinearSpace(true),
		// Disable the library's standard-Myers and LCS fallback paths:
		// - Standard Myers (< smallInputThreshold) is O((N+M)²) when inputs are asymmetric.
		// - LCS (> largeInputThreshold) is O(N*M) for the DP table.
		// By setting these to 0 and MaxLines respectively, only Hirschberg's
		// O(N+M) linear-space algorithm runs. Our MaxLines guard above ensures
		// inputs never exceed largeInputThreshold.
		myers.WithSmallInputThreshold(0),
		myers.WithLargeInputThreshold(MaxLines),
	)

	return differ.Diff(a, b)
}

// DiffWithCustomPaths is almost same as Diff, but allows to specify custom paths for the diff header.
func DiffWithCustomPaths(a, b, aPath, bPath string) (string, error) {
	diff, err := Diff(a, b)
	if err != nil {
		return "", err
	}

	if diff == "" {
		return "", nil
	}

	// patch the diff header to include the manifest path
	diff, ok := strings.CutPrefix(diff, "--- a\n+++ b\n")
	if !ok {
		return "", fmt.Errorf("unexpected diff format")
	}

	var sb strings.Builder

	sb.WriteString("--- ")
	sb.WriteString(aPath)
	sb.WriteString("\n+++ ")
	sb.WriteString(bPath)
	sb.WriteString("\n")
	sb.WriteString(diff)

	return sb.String(), nil
}
