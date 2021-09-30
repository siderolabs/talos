// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"path/filepath"
	"testing"
)

// This test mainly exist for easier debugging with debugger.
func TestProcessFile(t *testing.T) {
	inputFile := filepath.Join("..", "..", "pkg", "machinery", "config", "types", "v1alpha1", "v1alpha1_types.go")
	outputFile := filepath.Join(t.TempDir(), "out.go")
	typeName := "Configuration"
	processFile(inputFile, outputFile, typeName)
}
