// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package toml

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"

	"github.com/pelletier/go-toml/v2"

	"github.com/siderolabs/talos/pkg/machinery/config/merge"
)

// tomlDecodeFile decodes a TOML file into the provided destination, and returns a sha256 hash of the file content.
func tomlDecodeFile(path string, dest any) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	defer f.Close() //nolint:errcheck

	hash := sha256.New()

	err = toml.NewDecoder(io.TeeReader(f, hash)).Decode(dest)

	return hash.Sum(nil), err
}

// Merge several TOML documents in files into one.
//
// Merge process relies on generic map[string]any merge which might not always be correct.
//
// Merge returns a sha256 checksum of each file merged.
func Merge(parts []string) ([]byte, map[string][]byte, error) {
	merged := map[string]any{}
	checksums := make(map[string][]byte, len(parts))

	var header []byte

	for _, part := range parts {
		partial := map[string]any{}

		hash, err := tomlDecodeFile(part, &partial)
		if err != nil {
			return nil, nil, fmt.Errorf("error decoding %q: %w", part, err)
		}

		if err := merge.Merge(merged, partial); err != nil {
			return nil, nil, fmt.Errorf("error merging %q: %w", part, err)
		}

		header = fmt.Appendf(header, "## %s (sha256:%s)\n", part, hex.EncodeToString(hash))
		checksums[part] = hash
	}

	var out bytes.Buffer

	_, _ = out.Write(header)
	_ = out.WriteByte('\n')

	if err := toml.NewEncoder(&out).SetIndentTables(true).Encode(merged); err != nil {
		return nil, nil, fmt.Errorf("error encoding merged config: %w", err)
	}

	return out.Bytes(), checksums, nil
}
