// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package toml

import (
	"bytes"
	"fmt"

	"github.com/BurntSushi/toml"

	"github.com/siderolabs/talos/pkg/machinery/config/merge"
)

// Merge several TOML documents in files into one.
//
// Merge process relies on generic map[string]interface{} merge which might not always be correct.
func Merge(parts []string) ([]byte, error) {
	merged := map[string]interface{}{}

	var header []byte

	for _, part := range parts {
		partial := map[string]interface{}{}

		if _, err := toml.DecodeFile(part, &partial); err != nil {
			return nil, fmt.Errorf("error decoding %q: %w", part, err)
		}

		if err := merge.Merge(merged, partial); err != nil {
			return nil, fmt.Errorf("error merging %q: %w", part, err)
		}

		header = append(header, []byte(fmt.Sprintf("## %s\n", part))...)
	}

	var out bytes.Buffer

	_, _ = out.Write(header) //nolint:errcheck
	_ = out.WriteByte('\n')  //nolint:errcheck

	if err := toml.NewEncoder(&out).Encode(merged); err != nil {
		return nil, fmt.Errorf("error encoding merged config: %w", err)
	}

	return out.Bytes(), nil
}
