// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package extend

import (
	"bytes"
	"crypto"
)

type digest struct {
	alg  crypto.Hash
	hash []byte
}

func New(alg crypto.Hash) *digest {
	return &digest{
		alg:  alg,
		hash: bytes.Repeat([]byte{0x00}, alg.Size()),
	}
}

func (d *digest) Hash() []byte {
	return d.hash
}

func (d *digest) Extend(data []byte) {
	// create hash of incoming data
	hash := d.alg.New()
	hash.Write(data)
	hashSum := hash.Sum(nil)

	// extend hash with previous data and hashed incoming data
	newHash := d.alg.New()
	newHash.Write(d.hash)
	newHash.Write(hashSum)

	// set sum as new hash
	d.hash = newHash.Sum(nil)
}
