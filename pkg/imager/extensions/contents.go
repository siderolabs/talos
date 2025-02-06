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
	"strings"

	"github.com/siderolabs/talos/internal/pkg/extensions"
	"github.com/siderolabs/talos/pkg/machinery/imager/quirks"
)

func findExtensionsWithKernelModules(extensions []*extensions.Extension, quirks quirks.Quirks) []string {
	var modulesPath []string

	for _, ext := range extensions {
		if ext.ProvidesKernelModules(quirks) {
			modulesPath = append(modulesPath, ext.KernelModuleDirectory(quirks))
		}
	}

	return modulesPath
}

// buildInitramfsContents builds a list of files to be included into initramfs directly, bypassing extensions squashfs.
//
// Two listings are returned:
// - uncompressedListing is a list of files that should be included into initramfs uncompressed prepended as a first section
// - compressedListing is a list of files that should be included into initramfs compressed.
func buildInitramfsContents(path string) (compressedListing, uncompressedListing []byte, err error) {
	var compressedBuffer, uncompressedBuffer bytes.Buffer

	if err := buildInitramfsContentsRecursive(path, "", &compressedBuffer, &uncompressedBuffer); err != nil {
		return nil, nil, err
	}

	return compressedBuffer.Bytes(), uncompressedBuffer.Bytes(), nil
}

func buildInitramfsContentsRecursive(basePath, path string, compressedW, uncompressedW io.Writer) error {
	if path != "" {
		if path == "kernel" || strings.HasPrefix(path, "kernel/") {
			fmt.Fprintf(uncompressedW, "%s\n", path)
		} else {
			fmt.Fprintf(compressedW, "%s\n", path)
		}
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
		if err = buildInitramfsContentsRecursive(basePath, filepath.Join(path, item.Name()), compressedW, uncompressedW); err != nil {
			return err
		}
	}

	return nil
}
