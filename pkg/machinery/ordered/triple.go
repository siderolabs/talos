// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package ordered

// Triple is three element tuple of ordered values.
type Triple[T1, T2, T3 Ordered] struct {
	V1 T1
	V2 T2
	V3 T3
}

// MakeTriple creates a new Triple.
func MakeTriple[T1, T2, T3 Ordered](v1 T1, v2 T2, v3 T3) Triple[T1, T2, T3] {
	return Triple[T1, T2, T3]{
		V1: v1,
		V2: v2,
		V3: v3,
	}
}

// Compare returns an integer comparing two triples in natural order.
// The result will be 0 if t == other, -1 if t < other, and +1 if t > other.
func (t Triple[T1, T2, T3]) Compare(other Triple[T1, T2, T3]) int {
	if result := cmp(t.V1, other.V1); result != 0 {
		return result
	} else if result := cmp(t.V2, other.V2); result != 0 {
		return result
	}

	return cmp(t.V3, other.V3)
}

// MoreThan checks if current triple is bigger than the other.
func (t Triple[T1, T2, T3]) MoreThan(other Triple[T1, T2, T3]) bool {
	return t.Compare(other) == 1
}

// LessThan checks if current triple is lesser than the other.
func (t Triple[T1, T2, T3]) LessThan(other Triple[T1, T2, T3]) bool {
	return t.Compare(other) == -1
}

// Equal checks if current triple is equal to the other.
func (t Triple[T1, T2, T3]) Equal(other Triple[T1, T2, T3]) bool {
	return t.Compare(other) == 0
}
