// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package keys

import (
	"fmt"

	"github.com/talos-systems/go-smbios/smbios"
)

// NodeIDKeyHandler generates the key based on current node information
// and provided template string.
type NodeIDKeyHandler struct{}

// NewNodeIDKeyHandler creates new NodeIDKeyHandler.
func NewNodeIDKeyHandler() (*NodeIDKeyHandler, error) {
	return &NodeIDKeyHandler{}, nil
}

// GetKey implements KeyHandler interface.
func (h *NodeIDKeyHandler) GetKey(options ...KeyOption) ([]byte, error) {
	opts, err := NewDefaultOptions(options)
	if err != nil {
		return nil, err
	}

	s, err := smbios.New()
	if err != nil {
		return nil, err
	}

	machineUUID := s.SystemInformation.UUID

	if machineUUID == "" {
		return nil, fmt.Errorf("machine UUID is not populated %s", machineUUID)
	}

	// primitive entropy check
	counts := map[rune]int{}
	for _, s := range machineUUID {
		counts[s]++
		if counts[s] > len(machineUUID)/2 {
			return nil, fmt.Errorf("machine UUID %s entropy check failed", machineUUID)
		}
	}

	return []byte(machineUUID + opts.PartitionLabel), nil
}
