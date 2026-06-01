// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package configpatcher

import (
	jsonpatch "github.com/evanphx/json-patch"

	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
)

// configOrBytes encapsulates either unmarshaled config or raw byte representation.
type configOrBytes struct {
	marshaled []byte
	config    config.Provider
}

func (cb *configOrBytes) Bytes() ([]byte, error) {
	if cb.marshaled != nil {
		return cb.marshaled, nil
	}

	var err error

	cb.marshaled, err = cb.config.EncodeBytes(encoder.WithComments(encoder.CommentsDisabled))
	if err != nil {
		return nil, err
	}

	cb.config = nil

	return cb.marshaled, nil
}

func (cb *configOrBytes) Config() (config.Provider, error) {
	if cb.config != nil {
		return cb.config, nil
	}

	var err error

	cb.config, err = configloader.NewFromBytes(cb.marshaled)
	if err != nil {
		return nil, err
	}

	cb.marshaled = nil

	return cb.config, nil
}

// Input to the patch application process.
type Input interface {
	Config() (config.Provider, error)
	Bytes() ([]byte, error)
}

// WithConfig returns a new Input that wraps the given config.
func WithConfig(config config.Provider) Input {
	return &configOrBytes{config: config}
}

// WithBytes returns a new Input that wraps the given bytes.
func WithBytes(bytes []byte) Input {
	return &configOrBytes{marshaled: bytes}
}

// Output of patch application process.
type Output = Input

// Apply config patches to Talos machine config.
//
// Apply either JSON6902 or StrategicMergePatch.
//
// This method tries to minimize conversion between byte and unmarshalled
// config representation as much as possible.
func Apply(in Input, patches []Patch) (Output, error) {
	for _, patch := range patches {
		switch p := patch.(type) {
		case jsonpatch.Patch:
			bytes, err := in.Bytes()
			if err != nil {
				return nil, err
			}

			patched, err := JSON6902(bytes, p)
			if err != nil {
				return nil, err
			}

			in = WithBytes(patched)
		case StrategicMergePatch:
			cfg, err := in.Config()
			if err != nil {
				return nil, err
			}

			patched, err := StrategicMerge(cfg, p)
			if err != nil {
				return nil, err
			}

			in = WithConfig(patched)
		}
	}

	return in, nil
}
