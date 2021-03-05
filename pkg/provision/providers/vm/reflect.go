// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	yaml "gopkg.in/yaml.v3"

	"github.com/talos-systems/talos/pkg/provision"
)

// Reflect decode state file.
func (p *Provisioner) Reflect(ctx context.Context, clusterName, stateDirectory string) (provision.Cluster, error) {
	statePath := filepath.Join(stateDirectory, clusterName)

	st, err := os.Stat(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("cluster %q not found: %w", clusterName, err)
		}

		return nil, err
	}

	if !st.IsDir() {
		return nil, fmt.Errorf("state path %q is not a directory: %s", statePath, st.Mode())
	}

	stateFile, err := os.Open(filepath.Join(statePath, stateFileName))
	if err != nil {
		return nil, err
	}

	defer stateFile.Close() //nolint:errcheck

	state := &State{}

	if err = yaml.NewDecoder(stateFile).Decode(state); err != nil {
		return nil, fmt.Errorf("error unmarshalling state file: %w", err)
	}

	if state.ProvisionerName != p.Name {
		return nil, fmt.Errorf("cluster %q was created with different provisioner %q", clusterName, state.ProvisionerName)
	}

	state.statePath = statePath

	return state, nil
}
