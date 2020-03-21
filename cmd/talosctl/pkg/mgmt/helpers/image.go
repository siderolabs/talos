// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package helpers

import (
	"fmt"
	"os"

	"github.com/talos-systems/talos/pkg/version"
)

// DefaultImage appends default image version.
func DefaultImage(image string) string {
	return fmt.Sprintf("%s:%s", image, getEnv("TAG", version.Tag))
}

func getEnv(key, fallback string) string {
	if value, ok := os.LookupEnv(key); ok {
		return value
	}

	return fallback
}
