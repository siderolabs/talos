// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package kubeconfig provides Kubernetes config file handling.
package kubeconfig

import (
	"fmt"
	"os"
	"path/filepath"
)

// DefaultPath returns path to ~/.kube/config.
func DefaultPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(home, ".kube/config"), nil
}

// SinglePath parses KUBECONFIG and the default kubeconfig file location
// and ensures there is only one to return.
func SinglePath() (string, error) {
	envVarFilePaths := filepath.SplitList(os.Getenv("KUBECONFIG"))
	switch len(envVarFilePaths) {
	case 0:
		return DefaultPath()
	case 1:
		return envVarFilePaths[0], nil
	default:
		return "", fmt.Errorf("multiple kubeconfig files defined")
	}
}
