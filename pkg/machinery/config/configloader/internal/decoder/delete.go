// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package decoder

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
)

// AppendDeletesTo appends all delete selectors found in the given YAML node to the given destination slice.
func AppendDeletesTo(n *yaml.Node, dest []config.Document, idx int) (_ []config.Document, err error) {
	defer func() {
		if r := recover(); r != nil {
			if re, ok := r.(error); ok {
				err = re
			}
		}
	}()

	allDeletes(n)(func(path []string, elem delElem) bool {
		switch elem.parent.Kind {
		case yaml.DocumentNode:
			dest = append(dest, makeSelector(path, n.Content[0], idx, "", ""))
		case yaml.MappingNode:
			dest = append(dest, makeSelector(path, n.Content[0], idx, "", ""))
		case yaml.SequenceNode:
			dest = append(dest, makeSequenceSelector(path, n.Content[0], elem.node, idx))
		case yaml.ScalarNode, yaml.AliasNode:
		}

		return true
	})

	return dest, nil
}

func allDeletes(node *yaml.Node) func(yield func([]string, delElem) bool) {
	return func(yield func([]string, delElem) bool) {
		_, okToDel := processNode(nil, node, make([]string, 0, 8), yield)
		if okToDel {
			*node = yaml.Node{}
		}
	}
}

func makeSequenceSelector(path []string, root, node *yaml.Node, i int) Selector {
	if node.Kind != yaml.MappingNode {
		panic(errors.New("expected a mapping node"))
	}

	// map node inside sequence node, collect the first key:val aside from $patch:delete as selector
	for j := 0; j < len(node.Content)-1; j += 2 {
		key := node.Content[j]
		val := node.Content[j+1]

		if val.Kind == yaml.ScalarNode && key.Value == "$patch" && val.Value == "delete" {
			continue
		}

		return makeSelector(path, root, i, key.Value, val.Value)
	}

	panic(errors.New("no key:val found in sequence node for path " + strings.Join(path, ".")))
}

func makeSelector(path []string, root *yaml.Node, i int, key, val string) Selector {
	isRequired := len(path) == 0

	apiVersion := findValue(root, "apiVersion", isRequired)
	kind := findValue(root, "kind", isRequired)

	switch {
	case kind == "" && apiVersion == "":
		kind = v1alpha1.Version // legacy document
	case kind != "" && apiVersion != "":
	default:
		panic(fmt.Errorf("kind and apiVersion must be both set for path %s", strings.Join(path, ".")))
	}

	sel := selector{
		path:          slices.Clone(path),
		docIdx:        i,
		docAPIVersion: apiVersion,
		docKind:       kind,
		key:           key,
		value:         val,
	}

	switch name := findValue(root, "name", false); name {
	case "":
		return &sel
	default:
		return &namedSelector{
			selector: sel,
			name:     name,
		}
	}
}

type delElem struct {
	path         []string
	parent, node *yaml.Node
}

// processNode recursively processes a YAML node, searching for a "$patch: delete" nodes and calling the yield function
// with path for each one found.
//
//nolint:gocyclo,cyclop
func processNode(
	parent, v *yaml.Node,
	path []string,
	yield func(path []string, d delElem) bool,
) (bool, bool) {
	if v.Kind != yaml.DocumentNode && parent == nil {
		panic(errors.New("parent must be non-nil for non-document nodes"))
	}

	switch v.Kind {
	case yaml.DocumentNode:
		okToCont, okToDel := processNode(v, v.Content[0], path, yield)

		switch {
		case !okToCont:
			return false, okToDel
		case okToDel:
			return false, true
		default:
			return false, isEmptyDoc(v.Content[0])
		}

	case yaml.MappingNode:
		for i := 0; i < len(v.Content)-1; i += 2 {
			keyNode := v.Content[i]
			valueNode := v.Content[i+1]

			if valueNode.Kind == yaml.ScalarNode && keyNode.Value == "$patch" && valueNode.Value == "delete" {
				if parent.Kind != yaml.SequenceNode {
					ensureNoSeqInChain(path)
				}

				return yield(path, delElem{path: path, parent: parent, node: v}), true
			}

			okToCont, okToDel := processNode(v, valueNode, append(path, keyNode.Value), yield)
			if !okToCont {
				return false, okToDel
			} else if okToDel {
				v.Content = slices.Delete(v.Content, i, i+2)
				i -= 2

				if len(v.Content) == 0 {
					return true, true
				}
			}
		}
	case yaml.SequenceNode:
		for i := 0; i < len(v.Content); i++ {
			okToCont, okToDel := processNode(v, v.Content[i], append(path, "["+strconv.Itoa(i)+"]"), yield)
			if !okToCont {
				return false, okToDel
			} else if okToDel {
				v.Content = slices.Delete(v.Content, i, i+1)
				i--

				if len(v.Content) == 0 {
					return true, true
				}
			}
		}
	case yaml.ScalarNode, yaml.AliasNode:
	}

	return true, false
}

func isEmptyDoc(node *yaml.Node) bool {
	if node.Kind != yaml.MappingNode {
		return false
	}

	for i := 0; i < len(node.Content)-1; i += 2 {
		keyNode := node.Content[i]
		val := node.Content[i+1]

		if keyNode.Kind != yaml.ScalarNode || val.Kind != yaml.ScalarNode {
			return false
		}

		if keyNode.Value != "version" && keyNode.Value != "kind" && keyNode.Value != "name" {
			return false
		}
	}

	return true
}

func ensureNoSeqInChain(path []string) {
	for _, p := range path {
		if p[0] == '[' {
			panic(errors.New("cannot delete an inner key in '" + strings.Join(path, ".") + "'"))
		}
	}
}

func findValue(node *yaml.Node, key string, required bool) string {
	if node.Kind != yaml.MappingNode {
		panic(errors.New("expected a mapping node"))
	}

	for i := 0; i < len(node.Content)-1; i += 2 {
		keyNode := node.Content[i]
		val := node.Content[i+1]

		if keyNode.Kind == yaml.ScalarNode && keyNode.Value == key {
			return val.Value
		}
	}

	if required {
		panic(fmt.Errorf("missing %s in document for which $patch: delete is used", key))
	}

	return ""
}
