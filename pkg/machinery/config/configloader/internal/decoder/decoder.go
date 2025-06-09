// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package decoder provides a YAML decoder for machine configuration documents.
package decoder

import (
	"cmp"
	"errors"
	"fmt"
	"io"

	"github.com/siderolabs/gen/xyaml"
	yaml "gopkg.in/yaml.v3"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
)

// ErrMissingKind indicates that the manifest is missing a kind.
var ErrMissingKind = errors.New("missing kind")

const (
	// ManifestAPIVersionKey is the string indicating a manifest's version.
	ManifestAPIVersionKey = "apiVersion"
	// ManifestKindKey is the string indicating a manifest's kind.
	ManifestKindKey = "kind"
	// ManifestDeprecatedKeyMachine represents the deprecated v1alpha1 manifest.
	ManifestDeprecatedKeyMachine = "machine"
	// ManifestDeprecatedKeyCluster represents the deprecated v1alpha1 manifest.
	ManifestDeprecatedKeyCluster = "cluster"
	// ManifestDeprecatedKeyDebug represents the deprecated v1alpha1 manifest.
	ManifestDeprecatedKeyDebug = "debug"
	// ManifestDeprecatedKeyPersist represents the deprecated v1alpha1 manifest.
	ManifestDeprecatedKeyPersist = "persist"
)

// Decoder represents a multi-doc YAML decoder.
type Decoder struct{}

// Decode decodes all known manifests.
func (d *Decoder) Decode(r io.Reader, allowPatchDelete bool) ([]config.Document, error) {
	return parse(r, allowPatchDelete)
}

// NewDecoder initializes and returns a `Decoder`.
func NewDecoder() *Decoder {
	return &Decoder{}
}

type documentID struct {
	APIVersion string
	Kind       string
	Name       string
}

//nolint:gocyclo
func parse(r io.Reader, allowPatchDelete bool) (decoded []config.Document, err error) {
	// Recover from yaml.v3 panics because we rely on machine configuration loading _a lot_.
	defer func() {
		if p := recover(); p != nil {
			err = fmt.Errorf("recovered: %v", p)
		}
	}()

	decoded = []config.Document{}

	dec := yaml.NewDecoder(r)

	dec.KnownFields(true)

	knownDocuments := map[documentID]struct{}{}

	// Iterate through all defined documents.
	for i := 0; ; i++ {
		var manifests yaml.Node

		if err = dec.Decode(&manifests); err != nil {
			if errors.Is(err, io.EOF) {
				return decoded, nil
			}

			return nil, fmt.Errorf("decode error: %w", err)
		}

		if manifests.Kind != yaml.DocumentNode {
			return nil, errors.New("expected a document")
		}

		if allowPatchDelete {
			decoded, err = AppendDeletesTo(&manifests, decoded, i)
			if err != nil {
				return nil, err
			}

			if manifests.IsZero() {
				continue
			}
		}

		for _, manifest := range manifests.Content {
			id := documentID{
				APIVersion: findValue(manifest, ManifestAPIVersionKey, false),
				Kind:       cmp.Or(findValue(manifest, ManifestKindKey, false), "v1alpha1"),
				Name:       findValue(manifest, "name", false),
			}

			if _, ok := knownDocuments[id]; ok {
				return nil, fmt.Errorf("duplicate document %s/%s/%s is not allowed", id.APIVersion, id.Kind, id.Name)
			}

			knownDocuments[id] = struct{}{}

			var target config.Document

			if target, err = decode(manifest); err != nil {
				return nil, err
			}

			decoded = append(decoded, target)
		}
	}
}

//nolint:gocyclo
func decode(manifest *yaml.Node) (target config.Document, err error) {
	var (
		version string
		kind    string
	)

	for i, node := range manifest.Content {
		switch node.Value {
		case ManifestKindKey:
			if len(manifest.Content) < i+1 {
				return nil, errors.New("missing manifest content")
			}

			if err = manifest.Content[i+1].Decode(&kind); err != nil {
				return nil, fmt.Errorf("kind decode: %w", err)
			}
		case ManifestAPIVersionKey:
			if len(manifest.Content) < i+1 {
				return nil, errors.New("missing manifest content")
			}

			if err = manifest.Content[i+1].Decode(&version); err != nil {
				return nil, fmt.Errorf("version decode: %w", err)
			}
		case
			ManifestDeprecatedKeyMachine,
			ManifestDeprecatedKeyCluster,
			ManifestDeprecatedKeyDebug,
			ManifestDeprecatedKeyPersist:
			version = "v1alpha1"
		}
	}

	switch {
	case version == "v1alpha1" && kind == "":
		target, err = registry.New("v1alpha1", "")
	case kind == "":
		err = ErrMissingKind
	default:
		target, err = registry.New(kind, version)
	}

	if err != nil {
		return nil, err
	}

	if err = manifest.Decode(target); err != nil {
		return nil, fmt.Errorf("error decoding %s to %T: %w", kind, target, err)
	}

	if err = xyaml.CheckUnknownKeys(target, manifest); err != nil {
		return nil, err
	}

	return target, nil
}
