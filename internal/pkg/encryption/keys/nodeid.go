// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package keys

import (
	"context"
	"fmt"

	"github.com/siderolabs/go-blockdevice/v2/encryption"
	"github.com/siderolabs/go-blockdevice/v2/encryption/token"

	"github.com/siderolabs/talos/internal/pkg/encryption/helpers"
)

// NodeIDKeyHandler generates the key based on current node information
// and provided template string.
type NodeIDKeyHandler struct {
	KeyHandler

	partitionLabel string
	getSystemInfo  helpers.SystemInformationGetter
}

// NewNodeIDKeyHandler creates new NodeIDKeyHandler.
func NewNodeIDKeyHandler(key KeyHandler, partitionLabel string, systemInfoGetter helpers.SystemInformationGetter) *NodeIDKeyHandler {
	return &NodeIDKeyHandler{
		KeyHandler:     key,
		partitionLabel: partitionLabel,
		getSystemInfo:  systemInfoGetter,
	}
}

// NewKey implements Handler interface.
func (h *NodeIDKeyHandler) NewKey(ctx context.Context) (*encryption.Key, token.Token, error) {
	k, err := h.GetKey(ctx, nil)

	return k, nil, err
}

// GetKey implements Handler interface.
func (h *NodeIDKeyHandler) GetKey(ctx context.Context, _ token.Token) (*encryption.Key, error) {
	systemInformation, err := h.getSystemInfo(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get UUID: %w", err)
	}

	nodeUUID := systemInformation.TypedSpec().UUID

	if nodeUUID == "" {
		return nil, fmt.Errorf("machine UUID is not populated %s", nodeUUID)
	}

	// primitive entropy check
	counts := map[rune]int{}
	for _, s := range nodeUUID {
		counts[s]++
		if counts[s] > len(nodeUUID)/2 {
			return nil, fmt.Errorf("machine UUID %s entropy check failed", nodeUUID)
		}
	}

	return encryption.NewKey(h.slot, []byte(nodeUUID+h.partitionLabel)), nil
}
