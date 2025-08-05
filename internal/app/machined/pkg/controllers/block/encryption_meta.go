// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

import (
	"encoding/json"
	"fmt"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/types/block"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
)

// MarshalEncryptionMeta is a function to persist encryption config to the META value.
func MarshalEncryptionMeta(cfg config.EncryptionConfig) ([]byte, error) {
	return json.Marshal(cfg)
}

// UnmarshalEncryptionMeta is a function to load encryption config from the META value.
func UnmarshalEncryptionMeta(data []byte) (config.EncryptionConfig, error) {
	var encryptionFromMeta block.EncryptionSpec

	if err := json.Unmarshal(data, &encryptionFromMeta); err != nil {
		var legacyEncryption v1alpha1.EncryptionConfig

		if legacyErr := json.Unmarshal(data, &legacyEncryption); legacyErr != nil {
			return nil, fmt.Errorf("error unmarshalling state encryption meta key: %w", err)
		}

		return &legacyEncryption, nil
	}

	return &encryptionFromMeta, nil
}
