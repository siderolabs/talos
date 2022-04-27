// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package ordered_test

import (
	"math"
	"math/rand"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/talos-systems/talos/pkg/machinery/ordered"
)

func TestTriple(t *testing.T) {
	t.Parallel()

	expectedSlice := []ordered.Triple[int, string, float64]{
		ordered.MakeTriple(math.MinInt64, "Alpha", 69.0),
		ordered.MakeTriple(-200, "Alpha", 69.0),
		ordered.MakeTriple(-200, "Beta", -69.0),
		ordered.MakeTriple(-200, "Beta", 69.0),
		ordered.MakeTriple(1, "", 69.0),
		ordered.MakeTriple(1, "Alpha", 67.0),
		ordered.MakeTriple(1, "Alpha", 68.0),
		ordered.MakeTriple(10, "Alpha", 68.0),
		ordered.MakeTriple(10, "Beta", 68.0),
		ordered.MakeTriple(math.MaxInt64, "", 69.0),
	}

	seed := time.Now().Unix()
	rnd := rand.New(rand.NewSource(seed))

	for i := 0; i < 1000; i++ {
		a := append([]ordered.Triple[int, string, float64](nil), expectedSlice...)
		rnd.Shuffle(len(a), func(i, j int) { a[i], a[j] = a[j], a[i] })
		sort.Slice(a, func(i, j int) bool {
			return a[i].LessThan(a[j])
		})
		require.Equal(t, expectedSlice, a, "failed with seed %d iteration %d", seed, i)
	}
}
