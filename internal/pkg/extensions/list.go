// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package extensions

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"

	"github.com/siderolabs/talos/pkg/machinery/extensions"
)

// List prepared unpacked extensions under rootPath.
func List(rootPath string) ([]*Extension, error) {
	items, err := os.ReadDir(rootPath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}

		return nil, err
	}

	if len(items) == 0 {
		return nil, nil
	}

	slices.SortFunc(items, func(a, b os.DirEntry) int { return cmp.Compare(a.Name(), b.Name()) })

	result := make([]*Extension, 0, len(items))

	for _, item := range items {
		if !item.IsDir() {
			return nil, fmt.Errorf("unexpected non-directory entry: %q", item.Name())
		}

		ext, err := extensions.Load(filepath.Join(rootPath, item.Name()))
		if err != nil {
			return nil, fmt.Errorf("error loading extension %s: %w", item.Name(), err)
		}

		result = append(result, &Extension{ext})
	}

	return result, nil
}
