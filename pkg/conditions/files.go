// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package conditions

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"time"

	"github.com/siderolabs/gen/xslices"
)

type file string

func (filename file) Wait(ctx context.Context) error {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for {
		_, err := os.Stat(string(filename))
		if err == nil {
			return nil
		}

		if !errors.Is(err, fs.ErrNotExist) {
			return err
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (filename file) String() string {
	return fmt.Sprintf("file %q to exist", string(filename))
}

// WaitForFileToExist is a service condition that will wait for the existence of
// a file.
func WaitForFileToExist(filename string) Condition {
	return file(filename)
}

// WaitForFilesToExist is a service condition that will wait for the existence of all the files.
func WaitForFilesToExist(filenames ...string) Condition {
	conditions := xslices.Map(filenames, WaitForFileToExist)

	return WaitForAll(conditions...)
}
