// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1

import (
	"fmt"

	"gopkg.in/yaml.v2"
)

// Load config version v1alpha1.
func Load(data []byte) (config *Config, err error) {
	config = &Config{}
	if err = yaml.Unmarshal(data, config); err != nil {
		return config, fmt.Errorf("failed to parse v1alpha1 config: %w", err)
	}

	return config, nil
}
