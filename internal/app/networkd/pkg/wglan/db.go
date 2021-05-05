// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package wglan

import (
	"fmt"
	"sync"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

type peerDB struct {
	db map[wgtypes.Key]*PrePeer

	mu sync.RWMutex
}

// Get returns the PrePeer with the given Public Key, if it is in the database.
func (d *peerDB) Get(id wgtypes.Key) *PrePeer {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if d.db == nil {
		return nil
	}

	if p, ok := d.db[id]; ok {
		return p
	}

	return nil
}

// List returns the set of PrePeers from the database.
func (d *peerDB) List() (list []*PrePeer) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if d.db == nil {
		return nil
	}

	for _, p := range d.db {
		list = append(list, p)
	}

	return list
}

// Merge adds or merges the PrePeer information with any existing information in the database for that PrePeer.
func (d *peerDB) Merge(p *PrePeer) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if p == nil {
		return fmt.Errorf("empty prepeer")
	}

	if d.db == nil {
		d.db = make(map[wgtypes.Key]*PrePeer)
	}

	existing, ok := d.db[p.PublicKey]
	if !ok {
		d.db[p.PublicKey] = p

		return nil
	}

	if _, err := existing.Merge(p); err != nil {
		return fmt.Errorf("failed to merge pre-peer %q information: %w", p.PublicKey.String(), err)
	}

	return nil
}
