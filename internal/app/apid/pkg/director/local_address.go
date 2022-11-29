// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package director

import (
	"context"
	"sync"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/talos/pkg/machinery/resources/network"
)

// LocalAddressProvider provides local address information.
type LocalAddressProvider interface {
	IsLocalTarget(string) bool
}

// localAddressProvider watches and keeps track of the local node addresses.
type localAddressProvider struct {
	mu sync.Mutex

	localAddresses map[string]struct{}
	localHostnames map[string]struct{}
}

// NewLocalAddressProvider initializes and returns a new LocalAddressProvider.
func NewLocalAddressProvider(st state.State) (LocalAddressProvider, error) {
	p := &localAddressProvider{}

	evCh := make(chan state.Event)

	if err := st.Watch(context.Background(), resource.NewMetadata(network.NamespaceName, network.NodeAddressType, network.NodeAddressCurrentID, resource.VersionUndefined), evCh); err != nil {
		return nil, err
	}

	if err := st.Watch(context.Background(), resource.NewMetadata(network.NamespaceName, network.HostnameStatusType, network.HostnameID, resource.VersionUndefined), evCh); err != nil {
		return nil, err
	}

	go p.watch(evCh)

	return p, nil
}

func (p *localAddressProvider) watch(evCh <-chan state.Event) {
	for ev := range evCh {
		switch ev.Type {
		case state.Created, state.Updated:
			// expected
		case state.Destroyed, state.Bootstrapped, state.Errored:
			// shouldn't happen, ignore
			continue
		}

		switch r := ev.Resource.(type) {
		case *network.NodeAddress:
			p.mu.Lock()

			p.localAddresses = make(map[string]struct{}, len(r.TypedSpec().Addresses))

			for _, addr := range r.TypedSpec().Addresses {
				p.localAddresses[addr.Addr().String()] = struct{}{}
			}

			p.mu.Unlock()
		case *network.HostnameStatus:
			p.mu.Lock()

			p.localHostnames = make(map[string]struct{}, 2)

			p.localHostnames[r.TypedSpec().Hostname] = struct{}{}
			p.localHostnames[r.TypedSpec().FQDN()] = struct{}{}

			p.mu.Unlock()
		}
	}
}

// IsLocalTarget returns true if the address (hostname) is local.
func (p *localAddressProvider) IsLocalTarget(target string) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	_, ok1 := p.localAddresses[target]
	_, ok2 := p.localHostnames[target]

	return ok1 || ok2
}
