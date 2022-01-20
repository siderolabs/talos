// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package extensions

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
)

// List prepared unpacked extensions under rootPath.
func List(rootPath string) ([]*Extension, error) {
	items, err := os.ReadDir(rootPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}

		return nil, err
	}

	if len(items) == 0 {
		return nil, nil
	}

	sort.Slice(items, func(i, j int) bool { return items[i].Name() < items[j].Name() })

	result := make([]*Extension, 0, len(items))

	for _, item := range items {
		if !item.IsDir() {
			return nil, fmt.Errorf("unexpected non-directory entry: %q", item.Name())
		}

		ext, err := Load(filepath.Join(rootPath, item.Name()))
		if err != nil {
			return nil, fmt.Errorf("error loading extension %s: %w", item.Name(), err)
		}

		result = append(result, ext)
	}

	return result, nil
}
