// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"errors"
	"os"
	"path/filepath"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// Path represents a path to a configuration file.
type Path struct {
	// Path is the filesystem path of the config.
	Path string
	// WriteAllowed is true if the path is allowed to be written.
	WriteAllowed bool
}

// GetTalosDirectory returns path to Talos directory (~/.talos).
func GetTalosDirectory() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, constants.TalosDir), nil
}

// GetDefaultPaths returns the list of config file paths in order of priority.
func GetDefaultPaths() ([]Path, error) {
	talosDir, err := GetTalosDirectory()
	if err != nil {
		return nil, err
	}

	result := make([]Path, 0, 3)

	if path, ok := os.LookupEnv(constants.TalosConfigEnvVar); ok {
		result = append(result, Path{
			Path:         path,
			WriteAllowed: true,
		})
	}

	result = append(
		result,
		Path{
			Path:         filepath.Join(talosDir, constants.TalosconfigFilename),
			WriteAllowed: true,
		},
		Path{
			Path:         filepath.Join(constants.ServiceAccountMountPath, constants.TalosconfigFilename),
			WriteAllowed: false,
		},
	)

	return result, nil
}

// CustomSideroV1KeysDirPath returns the custom SideroV1 auth keys directory path if it's provided as command line flag or with environment variable.
func CustomSideroV1KeysDirPath(customPath string) string {
	if path, ok := os.LookupEnv(constants.SideroV1KeysDirEnvVar); ok {
		return path
	}

	if customPath != "" {
		return customPath
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(home, constants.TalosDir, constants.SideroV1KeysDir)
}

// firstValidPath iterates over the default paths and returns the first one that exists and readable.
// If no path is found, it will ensure that the first path that allows writes is created and returned.
// If no path is found that is writable, an error is returned.
func firstValidPath() (Path, error) {
	paths, err := GetDefaultPaths()
	if err != nil {
		return Path{}, err
	}

	var firstWriteAllowed Path

	for _, path := range paths {
		_, err = os.Stat(path.Path)
		if err == nil {
			return path, nil
		}

		if firstWriteAllowed.Path == "" && path.WriteAllowed {
			firstWriteAllowed = path
		}
	}

	if firstWriteAllowed.Path == "" {
		return Path{}, errors.New("no valid config paths found")
	}

	err = ensure(firstWriteAllowed.Path)
	if err != nil {
		return Path{}, err
	}

	return firstWriteAllowed, nil
}
