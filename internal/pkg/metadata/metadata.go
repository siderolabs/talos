// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package metadata

import (
	"errors"
	"io/ioutil"
	"path/filepath"
	"time"

	"github.com/talos-systems/talos/internal/pkg/runtime"
	"github.com/talos-systems/talos/pkg/constants"

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
func Open() (m *Metadata, err error) {
	m = &Metadata{}

	b, err := ioutil.ReadFile(m.Path())
	if err != nil {
		return nil, err
	}

	if len(b) == 0 {
		return nil, errors.New("metadata file is empty")
	}

	if err = yaml.Unmarshal(b, m); err != nil {
		return nil, err
	}

	return m, nil
}

// Save attempts to save the metadata.
func (m *Metadata) Save() (err error) {
	var b []byte

	if b, err = m.Bytes(); err != nil {
		return err
	}

	return ioutil.WriteFile(m.Path(), b, 0400)
}

// Bytes returns to byte slice representation of the metadata.
func (m *Metadata) Bytes() ([]byte, error) {
	return yaml.Marshal(m)
}

// Path returns the path to the metadata.
func (m *Metadata) Path() string {
	return filepath.Join(constants.BootMountPoint, constants.MetadataFile)
}
