/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package mount

// PointsIterator represents an iteratable group of mount points.
type PointsIterator struct {
	p       *Points
	value   *Point
	key     string
	index   int
	end     int
	err     error
	reverse bool
}

// Iter initializes and returns a mount point iterator.
func (p *Points) Iter() *PointsIterator {
	return &PointsIterator{
		p:     p,
		index: -1,
		end:   len(p.order) - 1,
		value: nil,
	}
}

// IterRev initializes and returns a mount point iterator that advances in
// reverse.
func (p *Points) IterRev() *PointsIterator {
	return &PointsIterator{
		p:       p,
		reverse: true,
		index:   len(p.points),
		end:     0,
		value:   nil,
	}
}

// Set sets an ordered value.
func (p *Points) Set(key string, value *Point) {
	if _, ok := p.points[key]; ok {
		for i := range p.order {
			if p.order[i] == key {
				p.order = append(p.order[:i], p.order[i+1:]...)
			}
		}
	}

	p.order = append(p.order, key)
	p.points[key] = value
}

// Get gets an ordered value.
func (p *Points) Get(key string) (value *Point, ok bool) {
	if value, ok = p.points[key]; ok {
		return value, true
	}

	return nil, false
}

// Key returns the current key.
func (i *PointsIterator) Key() string {
	return i.key
}

// Value returns current mount point.
func (i *PointsIterator) Value() *Point {
	if i.err != nil || i.index > len(i.p.points) {
		panic("invoked Value on expired iterator")
	}
	return i.value
}

// Err returns an error.
func (i *PointsIterator) Err() error {
	return i.err
}

// Next advances the iterator to the next value.
func (i *PointsIterator) Next() bool {
	if i.err != nil {
		return false
	}

	if i.reverse {
		i.index--
		if i.index < i.end {
			return false
		}
	} else {
		i.index++
		if i.index > i.end {
			return false
		}
	}

	i.key = i.p.order[i.index]
	i.value = i.p.points[i.key]

	if i.reverse {
		return i.index >= i.end
	}

	return i.index <= i.end
}
