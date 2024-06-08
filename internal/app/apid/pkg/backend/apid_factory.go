// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package backend

import (
	"crypto/tls"

	"github.com/siderolabs/gen/concurrent"
	"github.com/siderolabs/grpc-proxy/proxy"
)

// APIDFactory caches connection to apid instances by target.
//
// TODO: need to clean up idle connections from time to time.
type APIDFactory struct {
	cache    *concurrent.HashTrieMap[string, *APID]
	provider TLSConfigProvider
}

// TLSConfigProvider provides tls.Config for client connections.
type TLSConfigProvider interface {
	ClientConfig() (*tls.Config, error)
}

// NewAPIDFactory creates new APIDFactory with given tls.Config.
//
// Client TLS config is used to connect to other apid instances.
func NewAPIDFactory(provider TLSConfigProvider) *APIDFactory {
	return &APIDFactory{
		cache:    concurrent.NewHashTrieMap[string, *APID](),
		provider: provider,
	}
}

// Get backend by target.
//
// Get performs caching of backends.
func (factory *APIDFactory) Get(target string) (proxy.Backend, error) {
	b, ok := factory.cache.Load(target)
	if ok {
		return b, nil
	}

	backend, err := NewAPID(target, factory.provider.ClientConfig)
	if err != nil {
		return nil, err
	}

	existing, loaded := factory.cache.LoadOrStore(target, backend)
	if loaded {
		// race: another Get() call built different backend
		backend.Close()

		return existing, nil
	}

	return backend, nil
}

// Flush all cached backends.
//
// This ensures that all connections are closed.
func (factory *APIDFactory) Flush() {
	factory.cache.Enumerate(func(key string, backend *APID) bool {
		backend.Close()

		return true
	})
}
