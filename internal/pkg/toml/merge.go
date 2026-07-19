// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package toml

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"maps"
	"slices"

	"github.com/pelletier/go-toml/v2"

	"github.com/siderolabs/talos/pkg/machinery/config/merge"
)

// Part is a named TOML fragment with human-readable provenance.
type Part struct {
	Contents []byte
	Origin   string
}

// Merge merges named TOML configuration fragments in lexicographical order by name.
func Merge(parts map[string]Part) ([]byte, error) {
	merged := map[string]any{}

	var header []byte

	for _, name := range slices.Sorted(maps.Keys(parts)) {
		part := parts[name]
		partial := map[string]any{}

		if err := toml.Unmarshal(part.Contents, &partial); err != nil {
			return nil, fmt.Errorf("error decoding %q: %w", name, err)
		}

		if err := merge.Merge(merged, partial); err != nil {
			return nil, fmt.Errorf("error merging %q: %w", name, err)
		}

		hash := sha256.Sum256(part.Contents)
		header = fmt.Appendf(header, "## %s (sha256:%s)\n", part.Origin, hex.EncodeToString(hash[:]))
	}

	var out bytes.Buffer

	_, _ = out.Write(header)
	_ = out.WriteByte('\n')

	if err := toml.NewEncoder(&out).SetIndentTables(true).Encode(merged); err != nil {
		return nil, fmt.Errorf("error encoding merged config: %w", err)
	}

	return out.Bytes(), nil
}
