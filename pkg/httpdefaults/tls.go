// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package httpdefaults

import (
	"crypto/x509"
	"io/fs"
	"os"
	"sync"

	"github.com/talos-systems/talos/pkg/machinery/constants"
)

var (
	cachedPool *x509.CertPool
	cachedSt   fs.FileInfo
	cacheMu    sync.Mutex
)

// RootCAs provides a cached, but refreshed, list of root CAs.
//
// If loading certificates fails for any reason, function returns nil.
func RootCAs() *x509.CertPool {
	st, err := os.Stat(constants.DefaultTrustedCAFile)
	if err != nil {
		return nil
	}

	// check if the file hasn't changed
	cacheMu.Lock()
	defer cacheMu.Unlock()

	if cachedPool != nil && cachedSt != nil {
		if cachedSt.ModTime().Equal(st.ModTime()) && cachedSt.Size() == st.Size() {
			return cachedPool
		}
	}

	pool := x509.NewCertPool()

	contents, err := os.ReadFile(constants.DefaultTrustedCAFile)
	if err == nil {
		if pool.AppendCertsFromPEM(contents) {
			cachedPool = pool
			cachedSt = st
		}
	}

	// TODO: we should copy the pool here, but it's not possible until Go 1.19
	return cachedPool
}
