// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package textdiff_test

import (
	"fmt"
	"math/rand/v2"
	"runtime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/textdiff"
)

func TestDiff(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		a, b string

		expected string
	}{
		{
			name:     "empty",
			a:        "",
			b:        "",
			expected: "",
		},
		{
			name: "identical",
			a:    "line 1\nline 2\nline 3\n",
			b:    "line 1\nline 2\nline 3\n",

			expected: "",
		},
		{
			name: "completely different",
			a:    "line 1\nline 2\nline 3\n",
			b:    "line A\nline B\nline C\n",

			expected: `--- a
+++ b
@@ -1,3 +1,3 @@
-line 1
-line 2
-line 3
+line A
+line B
+line C
`,
		},
		{
			name: "inserted line",
			a:    "line 1\nline 3\n",
			b:    "line 1\nline 2\nline 3\n",

			expected: `--- a
+++ b
@@ -1,2 +1,3 @@
 line 1
+line 2
 line 3
`,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			diff, err := textdiff.Diff(test.a, test.b)
			require.NoError(t, err)
			assert.Equal(t, test.expected, diff)
		})
	}
}

func TestDiffWithCustomPaths(t *testing.T) {
	t.Parallel()

	diff, err := textdiff.DiffWithCustomPaths("line 1\nline 3\n", "line 1\nline 2\nline 3\n", "fileA.txt", "fileB.txt")
	require.NoError(t, err)
	assert.Equal(t, `--- fileA.txt
+++ fileB.txt
@@ -1,2 +1,3 @@
 line 1
+line 2
 line 3
`, diff)
}

func genRandomLines(n int) string {
	var sb strings.Builder

	for range n {
		fmt.Fprintf(&sb, "line %d\n", rand.Int())
	}

	return sb.String()
}

func TestDiffMemoryBudge(t *testing.T) {
	// not parallel, we need to measure memory allocations
	linesA := genRandomLines(1000)
	linesB := genRandomLines(20000)
	linesC := genRandomLines(1000)

	for _, test := range []struct {
		name string
		a, b string

		memoryBudget uint64
	}{
		{
			name: "empty",
			a:    "",
			b:    "",

			memoryBudget: 1024, // 1KB
		},
		{
			name: "large",
			a:    linesA + linesB,
			b:    linesB + linesC,

			memoryBudget: 3 * 1024 * 1024, // 3MB
		},
		{
			name: "large 2",
			a:    linesA + linesB,
			b:    linesA + linesB + linesC,

			memoryBudget: 2 * 1024 * 1024, // 2MB
		},
		{
			name: "completely different",
			a:    linesA,
			b:    linesC,

			memoryBudget: 512 * 1024, // 512KB
		},
		{
			name: "large to empty",

			a: linesA + linesB + linesC,
			b: "",

			memoryBudget: 6 * 1024 * 1024, // 6MB
		},
		{
			name: "empty to large",

			a: "",
			b: linesA + linesB + linesC,

			memoryBudget: 6 * 1024 * 1024, // 6MB
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			differ := textdiff.Diff

			allocs := MemoryAllocated(func() {
				_, err := differ(test.a, test.b)
				require.NoError(t, err)
			})

			require.LessOrEqual(t, allocs, test.memoryBudget, "memory allocations exceeded the budget")
		})
	}
}

func MemoryAllocated(f func()) uint64 {
	defer runtime.GOMAXPROCS(runtime.GOMAXPROCS(1))

	runtime.GC()

	// Measure the starting statistics
	var memstats runtime.MemStats
	runtime.ReadMemStats(&memstats)
	allocs := 0 - memstats.TotalAlloc

	f()

	// Read the final statistics
	runtime.ReadMemStats(&memstats)
	allocs += memstats.TotalAlloc

	return allocs
}
