// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package configloader provides methods to load Talos config.
package configloader

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	"github.com/talos-systems/talos/pkg/machinery/config"
	"github.com/talos-systems/talos/pkg/machinery/config/decoder"
	"github.com/talos-systems/talos/pkg/machinery/config/types/v1alpha1"
)

// ErrNoConfig is returned when no configuration was found in the input.
var ErrNoConfig = errors.New("config not found")

// newConfig initializes and returns a Configurator.
func newConfig(source []byte) (config config.Provider, err error) {
	dec := decoder.NewDecoder(source)

	manifests, err := dec.Decode()
	if err != nil {
		return nil, err
	}

	// Look for the older flat v1alpha1 file first, since we have to handle it in
	// a special way.
	for _, manifest := range manifests {
		if talosconfig, ok := manifest.(*v1alpha1.Config); ok {
			return v1alpha1.WrapReadonly(talosconfig, source), nil
		}
	}

	return nil, ErrNoConfig
}

// NewFromFile will take a filepath and attempt to parse a config file from it.
func NewFromFile(filepath string) (config.Provider, error) {
	source, err := fromFile(filepath)
	if err != nil {
		return nil, err
	}

	return newConfig(source)
}

// NewFromStdin initializes a config provider by reading from stdin.
func NewFromStdin() (config.Provider, error) {
	buf := bytes.NewBuffer(nil)

	_, err := io.Copy(buf, os.Stdin)
	if err != nil {
		return nil, err
	}

	config, err := NewFromBytes(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("failed load config from stdin: %w", err)
	}

	return config, nil
}

// NewFromBytes will take a byteslice and attempt to parse a config file from it.
func NewFromBytes(source []byte) (config.Provider, error) {
	return newConfig(source)
}

// fromFile is a convenience function that reads the config from disk.
func fromFile(p string) ([]byte, error) {
	return ioutil.ReadFile(p)
}
