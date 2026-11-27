// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package lvfs

import (
	"io"
	"slices"
	"strings"
)

// Catalog indexes the components of an LVFS metadata file by the firmware
// GUIDs they provide.
type Catalog struct {
	components []*Component
	byGUID     map[string][]int
}

// ParseCatalog reads AppStream metadata into a catalog.
//
// Only the components for which keep returns true are retained; a nil keep
// retains every component.
func ParseCatalog(r io.Reader, keep func(*Component) bool) (*Catalog, error) {
	c := &Catalog{byGUID: map[string][]int{}}

	err := ParseComponents(r, func(component *Component) error {
		if keep != nil && !keep(component) {
			return nil
		}

		index := len(c.components)
		c.components = append(c.components, component)

		for _, guid := range component.GUIDs() {
			guid = strings.ToLower(guid)
			c.byGUID[guid] = append(c.byGUID[guid], index)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return c, nil
}

// Len returns the number of components in the catalog.
func (c *Catalog) Len() int {
	return len(c.components)
}

// Lookup returns the components providing any of the GUIDs, each once, in
// catalog order.
func (c *Catalog) Lookup(guids []string) []*Component {
	var indexes []int

	for _, guid := range guids {
		for _, index := range c.byGUID[strings.ToLower(guid)] {
			if !slices.Contains(indexes, index) {
				indexes = append(indexes, index)
			}
		}
	}

	slices.Sort(indexes)

	components := make([]*Component, 0, len(indexes))

	for _, index := range indexes {
		components = append(components, c.components[index])
	}

	return components
}
