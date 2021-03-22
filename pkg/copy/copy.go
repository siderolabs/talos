// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package copy //nolint:predeclared

import (
	"io"
	"io/ioutil"
	"os"
	"path"
)

// File copies the `src` file to the `dst` file.
func File(src, dst string, setters ...Option) error {
	var (
		err     error
		s       *os.File
		d       *os.File
		info    os.FileInfo
		options Options
	)

	for _, setter := range setters {
		setter(&options)
	}

	if s, err = os.Open(src); err != nil {
		return err
	}

	//nolint:errcheck
	defer s.Close()

	if d, err = os.Create(dst); err != nil {
		return err
	}

	//nolint:errcheck
	defer d.Close()

	//nolint:errcheck
	defer d.Sync()

	if _, err = io.Copy(d, s); err != nil {
		return err
	}

	if info, err = os.Stat(src); err != nil {
		return err
	}

	mode := info.Mode()
	if options.Mode != 0 {
		mode = options.Mode
	}

	return os.Chmod(dst, mode)
}

// Dir copies the `src` directory to the `dst` directory.
func Dir(src, dst string, setters ...Option) error {
	var (
		err     error
		files   []os.FileInfo
		info    os.FileInfo
		options Options
	)

	for _, setter := range setters {
		setter(&options)
	}

	if info, err = os.Stat(src); err != nil {
		return err
	}

	mode := info.Mode()

	if options.Mode != 0 {
		mode = options.Mode
	}

	if err = os.MkdirAll(dst, mode); err != nil {
		return err
	}

	if files, err = ioutil.ReadDir(src); err != nil {
		return err
	}

	for _, file := range files {
		s := path.Join(src, file.Name())
		d := path.Join(dst, file.Name())

		if file.IsDir() {
			if err = Dir(s, d, setters...); err != nil {
				return err
			}
		} else {
			if err = File(s, d, setters...); err != nil {
				return err
			}
		}
	}

	return nil
}

// Option represents copy option.
type Option func(o *Options)

// Options represents copy options.
type Options struct {
	Mode os.FileMode
}

// WithMode sets destination files filemode.
func WithMode(m os.FileMode) Option {
	return func(o *Options) {
		o.Mode = m
	}
}
