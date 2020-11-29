// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package copy

import (
	"io"
	"io/ioutil"
	"os"
	"path"
)

// File copies the `src` file to the `dst` file.
func File(src, dst string) error {
	var (
		err  error
		s    *os.File
		d    *os.File
		info os.FileInfo
	)

	if s, err = os.Open(src); err != nil {
		return err
	}

	// nolint: errcheck
	defer s.Close()

	if d, err = os.Create(dst); err != nil {
		return err
	}

	// nolint: errcheck
	defer d.Close()

	// nolint: errcheck
	defer d.Sync()

	if _, err = io.Copy(d, s); err != nil {
		return err
	}

	if info, err = os.Stat(src); err != nil {
		return err
	}

	return os.Chmod(dst, info.Mode())
}

// Dir copies the `src` directory to the `dst` directory.
func Dir(src, dst string) error {
	var (
		err   error
		files []os.FileInfo
		info  os.FileInfo
	)

	if info, err = os.Stat(src); err != nil {
		return err
	}

	if err = os.MkdirAll(dst, info.Mode()); err != nil {
		return err
	}

	if files, err = ioutil.ReadDir(src); err != nil {
		return err
	}

	for _, file := range files {
		s := path.Join(src, file.Name())
		d := path.Join(dst, file.Name())

		if file.IsDir() {
			if err = Dir(s, d); err != nil {
				return err
			}
		} else {
			if err = File(s, d); err != nil {
				return err
			}
		}
	}

	return nil
}
