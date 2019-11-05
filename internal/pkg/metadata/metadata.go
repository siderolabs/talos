// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package metadata

import (
	"errors"
	"io/ioutil"
	"time"

	"github.com/talos-systems/talos/internal/pkg/runtime"

	"gopkg.in/yaml.v2"
)

// Metadata represents the node metadata.
type Metadata struct {
	Timestamp time.Time `yaml:"timestamp"`
	Upgraded  bool      `yaml:"upgraded"`
}

// NewMetadata initializes and returns the metadata.
func NewMetadata(sequence runtime.Sequence) *Metadata {
	upgraded := sequence == runtime.Upgrade

	return &Metadata{
		Timestamp: time.Now(),
		Upgraded:  upgraded,
	}
}

// Open attempts to read the metadata.
func Open(file string) (m *Metadata, err error) {
	b, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	if len(b) == 0 {
		return nil, errors.New("metadata file is empty")
	}

	m = &Metadata{}

	if err = yaml.Unmarshal(b, m); err != nil {
		return nil, err
	}

	return m, nil
}

// Bytes returns to byte slice representation of the metadata.
func (m *Metadata) Bytes() ([]byte, error) {
	return yaml.Marshal(m)
}
