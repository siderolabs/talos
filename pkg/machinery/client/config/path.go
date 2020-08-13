// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"os"
	"path/filepath"

	"github.com/talos-systems/talos/pkg/machinery/constants"
)

// GetTalosDirectory returns path to Talos directory (~/.talos).
func GetTalosDirectory() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".talos"), nil
}

// GetDefaultPath returns default path to Talos config.
func GetDefaultPath() (string, error) {
	if path, ok := os.LookupEnv(constants.TalosConfigEnvVar); ok {
		return path, nil
	}

	talosDir, err := GetTalosDirectory()
	if err != nil {
		return "", err
	}

	return filepath.Join(talosDir, "config"), nil
}
