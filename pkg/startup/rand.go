// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package startup

import (
	"encoding/binary"
	"fmt"
	"math/rand"

	cryptorand "crypto/rand"
)

// RandSeed default math/rand PRNG.
func RandSeed() error {
	seed := make([]byte, 8)
	if _, err := cryptorand.Read(seed); err != nil {
		return fmt.Errorf("error seeding rand: %w", err)
	}

	rand.Seed(int64(binary.LittleEndian.Uint64(seed)))

	return nil
}
