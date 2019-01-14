/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package conditions

import (
	"os"
	"time"
)

// ConditionFunc is the signature that all condition funcs must have.
type ConditionFunc = func() (bool, error)

// None is a service condition that has no conditions.
func None() ConditionFunc {
	return func() (bool, error) {
		return true, nil
	}
}

// FileExists is a service condition that checks for the existence of a file
// once and only once.
func FileExists(file string) ConditionFunc {
	return func() (bool, error) {
		_, err := os.Stat(file)
		if err != nil {
			if os.IsNotExist(err) {
				return false, nil
			}

			return false, err
		}

		return true, nil
	}
}

// WaitForFileToExist is a service condition that will wait for the existence of
// a file.
func WaitForFileToExist(file string) ConditionFunc {
	return func() (bool, error) {
		for {
			exists, err := FileExists(file)()
			if err != nil {
				return false, err
			}

			if exists {
				return true, nil
			}
			time.Sleep(1 * time.Second)
		}
	}
}

// WaitForFilesToExist is a service condition that will wait for the existence a
// set of files.
func WaitForFilesToExist(files ...string) ConditionFunc {
	return func() (exists bool, err error) {
	L:
		for {
			for _, f := range files {
				exists, err = FileExists(f)()
				if err != nil {
					return false, err
				}
				if !exists {
					time.Sleep(1 * time.Second)
					continue L
				}
			}
			if exists {
				return true, nil
			}
			time.Sleep(1 * time.Second)
		}
	}
}
