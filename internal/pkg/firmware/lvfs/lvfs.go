// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package lvfs implements LVFS (Linux Vendor Firmware Service) metadata handling:
// the trusted signing CA and the AppStream firmware metadata parser.
package lvfs

import (
	"crypto/x509"
	_ "embed"
	"sync"
)

// lvfsCA is the LVFS metadata signing CA, from https://github.com/fwupd/fwupd/blob/main/data/pki/LVFS-CA.pem.
//
//go:embed LVFS-CA.pem
var lvfsCA []byte

// CertPool returns the trusted roots for LVFS metadata signature verification.
var CertPool = sync.OnceValue(func() *x509.CertPool {
	pool := x509.NewCertPool()

	if !pool.AppendCertsFromPEM(lvfsCA) {
		panic("failed to parse embedded LVFS CA certificate")
	}

	return pool
})
