// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package utils

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/siderolabs/gen/pair/ordered"
)

// CopyInstruction describes a file copy operation.
type CopyInstruction = ordered.Pair[string, string]

// SourceDestination returns a CopyInstruction that copies src to dest.
func SourceDestination(src, dest string) CopyInstruction {
	return ordered.MakePair[string, string](src, dest)
}

// CopyFiles copies files according to the given instructions.
func CopyFiles(printf func(string, ...any), instructions ...CopyInstruction) error {
	for _, instruction := range instructions {
		if err := func(instruction CopyInstruction) error {
			src, dest := instruction.F1, instruction.F2

			if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
				return err
			}

			printf("copying %s to %s", src, dest)

			from, err := os.Open(src)
			if err != nil {
				return err
			}
			//nolint:errcheck
			defer from.Close()

			to, err := os.OpenFile(dest, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o666)
			if err != nil {
				return err
			}
			//nolint:errcheck
			defer to.Close()

			_, err = io.Copy(to, from)

			return err
		}(instruction); err != nil {
			return fmt.Errorf("error copying %s -> %s: %w", instruction.F1, instruction.F2, err)
		}
	}

	return nil
}
