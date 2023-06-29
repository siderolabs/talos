// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package keys

import (
	"context"
	"fmt"

	"github.com/siderolabs/go-blockdevice/blockdevice/encryption"
	"github.com/siderolabs/go-blockdevice/blockdevice/encryption/token"
)

// NodeIDKeyHandler generates the key based on current node information
// and provided template string.
type NodeIDKeyHandler struct {
	KeyHandler
	partitionLabel string
	nodeUUID       string
}

// NewNodeIDKeyHandler creates new NodeIDKeyHandler.
func NewNodeIDKeyHandler(key KeyHandler, partitionLabel, nodeUUID string) *NodeIDKeyHandler {
	return &NodeIDKeyHandler{
		KeyHandler:     key,
		partitionLabel: partitionLabel,
	}
}

// NewKey implements Handler interface.
func (h *NodeIDKeyHandler) NewKey(ctx context.Context) (*encryption.Key, token.Token, error) {
	k, err := h.GetKey(ctx, nil)

	return k, nil, err
}

// GetKey implements Handler interface.
func (h *NodeIDKeyHandler) GetKey(context.Context, token.Token) (*encryption.Key, error) {
	if h.nodeUUID == "" {
		return nil, fmt.Errorf("machine UUID is not populated %s", h.nodeUUID)
	}

	// primitive entropy check
	counts := map[rune]int{}
	for _, s := range h.nodeUUID {
		counts[s]++
		if counts[s] > len(h.nodeUUID)/2 {
			return nil, fmt.Errorf("machine UUID %s entropy check failed", h.nodeUUID)
		}
	}

	return encryption.NewKey(h.slot, []byte(h.nodeUUID+h.partitionLabel)), nil
}
