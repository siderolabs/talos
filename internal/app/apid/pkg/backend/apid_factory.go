// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package backend

import (
	"crypto/tls"
	"sync"

	"github.com/talos-systems/grpc-proxy/proxy"
	"google.golang.org/grpc/credentials"
)

// APIDFactory caches connection to apid instances by target.
//
// TODO: need to clean up idle connections from time to time.
type APIDFactory struct {
	cache sync.Map
	creds credentials.TransportCredentials
}

// NewAPIDFactory creates new APIDFactory with given tls.Config.
//
// Client TLS config is used to connect to other apid instances.
func NewAPIDFactory(config *tls.Config) *APIDFactory {
	return &APIDFactory{
		creds: credentials.NewTLS(config),
	}
}

// Get backend by target.
//
// Get performs caching of backends.
func (factory *APIDFactory) Get(target string) (proxy.Backend, error) {
	b, ok := factory.cache.Load(target)
	if ok {
		return b.(proxy.Backend), nil
	}

	backend, err := NewAPID(target, factory.creds)
	if err != nil {
		return nil, err
	}

	existing, loaded := factory.cache.LoadOrStore(target, backend)
	if loaded {
		// race: another Get() call built different backend
		backend.Close()

		return existing.(proxy.Backend), nil
	}

	return backend, nil
}
