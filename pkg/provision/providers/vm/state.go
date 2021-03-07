// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package vm

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/containernetworking/cni/libcni"
	yaml "gopkg.in/yaml.v3"

	"github.com/talos-systems/talos/pkg/provision"
)

// State common state representation for vm provisioners.
type State struct {
	ProvisionerName string
	BridgeName      string

	ClusterInfo provision.ClusterInfo

	VMCNIConfig *libcni.NetworkConfigList

	statePath string
}

// NewState create new vm provisioner state.
func NewState(statePath, provisionerName, clusterName string) (*State, error) {
	s := &State{
		ProvisionerName: provisionerName,
		statePath:       statePath,
	}

	_, err := os.Stat(s.statePath)

	if err == nil {
		return nil, fmt.Errorf(
			"state directory %q already exists, is the cluster %q already running? remove cluster state with talosctl cluster destroy",
			s.statePath,
			clusterName,
		)
	}

	if !os.IsNotExist(err) {
		return nil, fmt.Errorf("error checking state directory: %w", err)
	}

	if err = os.MkdirAll(s.statePath, os.ModePerm); err != nil {
		return nil, fmt.Errorf("error creating state directory: %w", err)
	}

	return s, nil
}

// Provisioner get provisioner name.
func (s *State) Provisioner() string {
	return s.ProvisionerName
}

// Info get cluster info.
func (s *State) Info() provision.ClusterInfo {
	return s.ClusterInfo
}

// StatePath get state config file path.
func (s *State) StatePath() (string, error) {
	if s.statePath == "" {
		return "", fmt.Errorf("state path is not set")
	}

	return s.statePath, nil
}

// Save save state to config file.
func (s *State) Save() error {
	// save state
	stateFile, err := os.Create(filepath.Join(s.statePath, stateFileName))
	if err != nil {
		return err
	}

	defer stateFile.Close() //nolint:errcheck

	if err = yaml.NewEncoder(stateFile).Encode(&s); err != nil {
		return fmt.Errorf("error marshaling state: %w", err)
	}

	return stateFile.Close()
}

// GetRelativePath get file path relative to config folder.
func (s *State) GetRelativePath(path string) string {
	return filepath.Join(s.statePath, path)
}
