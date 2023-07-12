// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package pcr

import (
	"crypto"
)

// Digest implements the PCR extension algorithm.
//
// Each time `Extend` is called, the hash of the previous data is
// prepended to the hash of new data and hashed together.
//
// The initial hash value is all zeroes.
type Digest struct {
	alg  crypto.Hash
	hash []byte
}

// NewDigest creates a new Digest with the speified hash algorithm.
func NewDigest(alg crypto.Hash) *Digest {
	return &Digest{
		alg:  alg,
		hash: make([]byte, alg.Size()),
	}
}

// Hash returns the current hash value.
func (d *Digest) Hash() []byte {
	return d.hash
}

// Extend extends the current hash with the specified data.
func (d *Digest) Extend(data []byte) {
	// create hash of incoming data
	hash := d.alg.New()
	hash.Write(data)
	hashSum := hash.Sum(nil)

	// extend hash with previous data and hashed incoming data
	hash.Reset()
	hash.Write(d.hash)
	hash.Write(hashSum)

	// set sum as new hash
	d.hash = hash.Sum(nil)
}
