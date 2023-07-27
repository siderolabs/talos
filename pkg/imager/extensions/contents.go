// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package extensions

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/siderolabs/talos/internal/pkg/extensions"
)

func findExtensionsWithKernelModules(extensions []*extensions.Extension) []string {
	var modulesPath []string

	for _, ext := range extensions {
		if ext.ProvidesKernelModules() {
			modulesPath = append(modulesPath, ext.KernelModuleDirectory())
		}
	}

	return modulesPath
}

func buildContents(path string) (io.Reader, error) {
	var listing bytes.Buffer

	if err := buildContentsRecursive(path, "", &listing); err != nil {
		return nil, err
	}

	return &listing, nil
}

func buildContentsRecursive(basePath, path string, w io.Writer) error {
	if path != "" {
		fmt.Fprintf(w, "%s\n", path)
	}

	st, err := os.Stat(filepath.Join(basePath, path))
	if err != nil {
		return err
	}

	if !st.IsDir() {
		return nil
	}

	contents, err := os.ReadDir(filepath.Join(basePath, path))
	if err != nil {
		return err
	}

	for _, item := range contents {
		if err = buildContentsRecursive(basePath, filepath.Join(path, item.Name()), w); err != nil {
			return err
		}
	}

	return nil
}
