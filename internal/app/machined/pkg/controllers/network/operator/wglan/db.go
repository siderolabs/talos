// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package wglan

import (
	"fmt"
	"sync"

	"golang.zx2c4.com/wireguard/wgctrl/wgtypes"
)

// PeerDB implements a local, internal Peer database.
type PeerDB struct {
	db map[string]*Peer

	mu sync.RWMutex
}

// Get returns the Peer with the given Public Key, if it is in the database.
func (d *PeerDB) Get(id wgtypes.Key) *Peer {
	if d == nil {
		return nil
	}

	d.mu.RLock()
	defer d.mu.RUnlock()

	if d.db == nil {
		return nil
	}

	if p, ok := d.db[id.String()]; ok {
		return p
	}

	return nil
}

// List returns the set of Peers from the database.
func (d *PeerDB) List() (list []*Peer) {
	if d == nil {
		return nil
	}

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

// Merge adds or merges the Peer information with any existing information in the database for that Peer.
func (d *PeerDB) Merge(p *Peer) error {
	if d == nil {
		return fmt.Errorf("empty PeerDB")
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	if p == nil {
		return fmt.Errorf("empty Peer")
	}

	if d.db == nil {
		d.db = make(map[string]*Peer)
	}

	existing, ok := d.db[p.PublicKey()]
	if !ok {
		d.db[p.PublicKey()] = p

		return nil
	}

	if err := existing.Merge(p); err != nil {
		return fmt.Errorf("failed to merge pre-peer %q information: %w", p.PublicKey(), err)
	}

	return nil
}
