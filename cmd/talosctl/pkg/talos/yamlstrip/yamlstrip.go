// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package yamlstrip provides YAML file manipulation.
package yamlstrip

import (
	"bytes"
	"errors"
	"io"

	"gopkg.in/yaml.v3"
)

// Comments strips comments from a YAML file.
//
// If the YAML file is parseable, it will be accurately stripped. Otherwise, it
// will be stripped in a best-effort manner.
func Comments(b []byte) []byte {
	stripped, err := stripViaDecoding(b)
	if err != nil {
		stripped = stripManual(b)
	}

	return stripped
}

func stripViaDecoding(b []byte) ([]byte, error) {
	var out bytes.Buffer

	decoder := yaml.NewDecoder(bytes.NewReader(b))
	encoder := yaml.NewEncoder(&out)

	for {
		var node yaml.Node

		err := decoder.Decode(&node)
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return nil, err
		}

		removeComments(&node)

		if err = encoder.Encode(&node); err != nil {
			return nil, err
		}
	}

	return out.Bytes(), nil
}

func removeComments(node *yaml.Node) {
	node.FootComment = ""
	node.HeadComment = ""
	node.LineComment = ""

	for _, child := range node.Content {
		removeComments(child)
	}
}

func stripManual(b []byte) []byte {
	var stripped []byte

	lines := bytes.Split(b, []byte("\n"))

	for i, line := range lines {
		trimline := bytes.TrimSpace(line)

		// this is not accurate, but best effort
		if bytes.HasPrefix(trimline, []byte("#")) && !bytes.HasPrefix(trimline, []byte("#!")) {
			continue
		}

		stripped = append(stripped, line...)

		if i < len(lines)-1 {
			stripped = append(stripped, '\n')
		}
	}

	return stripped
}
