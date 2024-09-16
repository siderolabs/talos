// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package cgroupsprinter provides functions to print cgroup information.
package cgroupsprinter

import (
	"fmt"
	"io"
	"slices"
	"strings"

	"github.com/siderolabs/talos/internal/pkg/cgroups"
)

type edgeType string

const (
	edgeTypeNone edgeType = ""
	edgeTypeLink edgeType = "│"
	edgeTypeMid  edgeType = "├──"
	edgeTypeEnd  edgeType = "└──"

	indentSize = 3
)

// PrintNode prints the cgroup node recursively.
//
//nolint:gocyclo
func PrintNode(name string, w io.Writer, schema *Schema, node, parent *cgroups.Node, level int, levelsEnded []int, lastNode, treeRoot bool) error {
	var prefix string

	for i := range level {
		if slices.Index(levelsEnded, i) != -1 {
			prefix += strings.Repeat(" ", indentSize+1)
		} else {
			prefix += string(edgeTypeLink) + strings.Repeat(" ", indentSize)
		}
	}

	var edge edgeType

	switch {
	case treeRoot:
		edge = edgeTypeNone
	case lastNode:
		edge = edgeTypeEnd
	default:
		edge = edgeTypeMid
	}

	rowData, err := schema.Render(struct {
		*cgroups.Node
		Parent *cgroups.Node
	}{
		Node:   node,
		Parent: parent,
	})
	if err != nil {
		return err
	}

	_, err = fmt.Fprintf(w, "%s%s%s\t%s\n", prefix, edge, name, rowData)
	if err != nil {
		return err
	}

	children := node.SortedChildren()

	if lastNode {
		levelsEnded = append(levelsEnded, level)
	}

	if !treeRoot {
		level++
	}

	for i, child := range children {
		last := i == len(children)-1

		if err = PrintNode(child, w, schema, node.Children[child], node, level, levelsEnded, last, false); err != nil {
			return err
		}
	}

	return nil
}
