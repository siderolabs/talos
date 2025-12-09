// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package provision

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/containernetworking/cni/libcni"
	yaml "go.yaml.in/yaml/v4"
)

// StateFileName is the name of the yaml state file.
const StateFileName = "state.yaml"

// State common state representation for vm provisioners.
type State struct {
	ProvisionerName string
	BridgeName      string

	ClusterInfo ClusterInfo

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

	if !errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("error checking state directory: %w", err)
	}

	if err = os.MkdirAll(s.statePath, os.ModePerm); err != nil {
		return nil, fmt.Errorf("error creating state directory: %w", err)
	}

	return s, nil
}

// ReadState reads and parses the state saved to a file.
func ReadState(ctx context.Context, clusterName, stateDirectory string) (*State, error) {
	statePath := filepath.Join(stateDirectory, clusterName)

	st, err := os.Stat(statePath)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, fmt.Errorf("cluster %q not found: %w", clusterName, err)
		}

		return nil, err
	}

	if !st.IsDir() {
		return nil, fmt.Errorf("state path %q is not a directory: %s", statePath, st.Mode())
	}

	stateFile, err := os.Open(filepath.Join(statePath, StateFileName))
	if err != nil {
		return nil, err
	}

	defer stateFile.Close() //nolint:errcheck

	state := &State{}

	if err = yaml.NewDecoder(stateFile).Decode(state); err != nil {
		return nil, fmt.Errorf("error unmarshalling state file: %w", err)
	}

	state.statePath = statePath

	return state, nil
}

// Provisioner get provisioner name.
func (s *State) Provisioner() string {
	return s.ProvisionerName
}

// Info get cluster info.
func (s *State) Info() ClusterInfo {
	return s.ClusterInfo
}

// StatePath get state config file path.
func (s *State) StatePath() (string, error) {
	if s.statePath == "" {
		return "", errors.New("state path is not set")
	}

	return s.statePath, nil
}

// Save save state to config file.
func (s *State) Save() error {
	// save state
	stateFile, err := os.Create(filepath.Join(s.statePath, StateFileName))
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

// GetShmPath get shm path.
func (s *State) GetShmPath(path string) string {
	if s.isDevShmAvailable() {
		return filepath.Join("/dev/shm", path)
	}

	return filepath.Join("/tmp", path)
}
