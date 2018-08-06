package conditions

import (
	"os"
	"time"
)

// None is a service condition that has no conditions.
func None() func() (bool, error) {
	return func() (bool, error) {
		return true, nil
	}
}

// FileExists is a service condition that checks for the existence of a file
// once and only once.
func FileExists(file string) func() (bool, error) {
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

// WaitForFileExists is a service condition that will wait for the existence of
// a file.
func WaitForFileExists(file string) func() (bool, error) {
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
func WaitForFilesToExist(files ...string) func() (bool, error) {
	return func() (exist bool, err error) {
		for {
			for _, f := range files {
				exist, err = FileExists(f)()
				if err != nil {
					return false, err
				}
			}
			if exist {
				return true, nil
			}
			time.Sleep(1 * time.Second)
		}
	}
}
