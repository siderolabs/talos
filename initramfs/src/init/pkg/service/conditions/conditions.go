package conditions

import (
	"os"
	"time"
)

func None() func() (bool, error) {
	return func() (bool, error) {
		return true, nil
	}
}

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
