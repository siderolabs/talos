// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package ordered

// Ordered is a constraint that permits any ordered type: any type
// that supports the operators < <= >= >.
type Ordered interface {
	~int | ~int8 | ~int16 | ~int32 | ~int64 | ~uint | ~uint8 | ~uint16 | ~uint32 | ~uint64 | ~uintptr | ~float32 | ~float64 | ~string
}

// Pair is two element tuple of ordered values.
type Pair[T1, T2 Ordered] struct {
	F1 T1
	F2 T2
}

// MakePair creates a new Pair.
func MakePair[T1, T2 Ordered](v1 T1, v2 T2) Pair[T1, T2] {
	return Pair[T1, T2]{
		F1: v1,
		F2: v2,
	}
}

// Compare returns an integer comparing two pairs in natural order.
// The result will be 0 if p == other, -1 if p < other, and +1 if p > other.
func (p Pair[T1, T2]) Compare(other Pair[T1, T2]) int {
	if result := cmp(p.F1, other.F1); result != 0 {
		return result
	}

	return cmp(p.F2, other.F2)
}

// MoreThan checks if current pair is bigger than the other.
func (p Pair[T1, T2]) MoreThan(other Pair[T1, T2]) bool {
	return p.Compare(other) == 1
}

// LessThan checks if current pair is lesser than the other.
func (p Pair[T1, T2]) LessThan(other Pair[T1, T2]) bool {
	return p.Compare(other) == -1
}

// Equal checks if current pair is equal to the other.
func (p Pair[T1, T2]) Equal(other Pair[T1, T2]) bool {
	return p.Compare(other) == 0
}

func cmp[T Ordered](a, b T) int {
	switch {
	case a == b:
		return 0
	case a < b:
		return -1
	default:
		return +1
	}
}
