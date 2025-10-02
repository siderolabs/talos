// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package artifacts

import (
	"archive/tar"
	"bufio"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/google/go-containerregistry/pkg/name"
	"gopkg.in/yaml.v3"

	"github.com/siderolabs/talos/pkg/images"
)

// ExtensionRef is a ref to the extension for some Talos version.
type ExtensionRef struct {
	TaggedReference name.Tag
	Digest          string
	Description     string
	Author          string

	imageDigest string
}

// OverlayRef is a ref to the overlay for some Talos version.
type OverlayRef struct {
	Name            string
	TaggedReference name.Tag
	Digest          string
}

// FetchOfficialExtensions fetches list of extensions for specific Talos version.
func FetchOfficialExtensions(tag string) ([]ExtensionRef, error) {
	var extensions []ExtensionRef

	m, err := newManager()
	if err != nil {
		return nil, err
	}

	if err := m.fetchImageByTag(images.DefaultExtensionsManifestRepository, tag, imageExportHandler(func(r io.Reader) error {
		var extractErr error

		extensions, extractErr = extractExtensionList(r)

		return extractErr
	})); err != nil {
		return nil, err
	}

	return extensions, nil
}

// FetchOfficialOverlays fetches list of overlays for specific Talos version.
func FetchOfficialOverlays(tag string) ([]OverlayRef, error) {
	var overlays []OverlayRef

	m, err := newManager()
	if err != nil {
		return nil, err
	}

	if err := m.fetchImageByTag(images.DefaultOverlaysManifestRepository, tag, imageExportHandler(func(r io.Reader) error {
		var extractErr error

		overlays, extractErr = extractOverlayList(r)

		return extractErr
	})); err != nil {
		return nil, err
	}

	return overlays, nil
}

type extensionsDescriptions map[string]struct {
	Author      string `yaml:"author"`
	Description string `yaml:"description"`
}

type overlaysDescriptions struct {
	Overlays []overlaysDescription `yaml:"overlays"`
}

type overlaysDescription struct {
	Name   string `yaml:"name"`
	Image  string `yaml:"image"`
	Digest string `yaml:"digest"`
}

//nolint:gocyclo
func extractExtensionList(r io.Reader) ([]ExtensionRef, error) {
	var extensions []ExtensionRef

	tr := tar.NewReader(r)

	var descriptions extensionsDescriptions

	for {
		hdr, err := tr.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return nil, fmt.Errorf("error reading tar header: %w", err)
		}

		if hdr.Name == "descriptions.yaml" {
			decoder := yaml.NewDecoder(tr)

			if err = decoder.Decode(&descriptions); err != nil {
				return nil, fmt.Errorf("error reading descriptions.yaml file: %w", err)
			}
		}

		if hdr.Name == "image-digests" {
			scanner := bufio.NewScanner(tr)

			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())

				tagged, digest, ok := strings.Cut(line, "@")
				if !ok {
					continue
				}

				taggedRef, err := name.NewTag(tagged)
				if err != nil {
					return nil, fmt.Errorf("failed to parse tagged reference %s: %w", tagged, err)
				}

				extensions = append(extensions, ExtensionRef{
					TaggedReference: taggedRef,
					Digest:          digest,

					imageDigest: line,
				})
			}

			if scanner.Err() != nil {
				return nil, fmt.Errorf("error reading image-digests: %w", scanner.Err())
			}
		}
	}

	if extensions != nil {
		if descriptions != nil {
			for i, extension := range extensions {
				desc, ok := descriptions[extension.imageDigest]
				if !ok {
					continue
				}

				extensions[i].Author = desc.Author
				extensions[i].Description = desc.Description
			}
		}

		return extensions, nil
	}

	return nil, errors.New("failed to find image-digests file")
}

func extractOverlayList(r io.Reader) ([]OverlayRef, error) {
	var overlays []OverlayRef

	tr := tar.NewReader(r)

	var overlayInfo overlaysDescriptions

	for {
		hdr, err := tr.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}

			return nil, fmt.Errorf("error reading tar header: %w", err)
		}

		if hdr.Name == "overlays.yaml" {
			decoder := yaml.NewDecoder(tr)

			if err = decoder.Decode(&overlayInfo); err != nil {
				return nil, fmt.Errorf("error reading overlays.yaml file: %w", err)
			}

			for _, overlay := range overlayInfo.Overlays {
				taggedRef, err := name.NewTag(overlay.Image)
				if err != nil {
					return nil, fmt.Errorf("failed to parse tagged reference %s: %w", overlay.Image, err)
				}

				overlays = append(overlays, OverlayRef{
					Name:            overlay.Name,
					TaggedReference: taggedRef,
					Digest:          overlay.Digest,
				})
			}
		}
	}

	if overlays != nil {
		return overlays, nil
	}

	return nil, errors.New("failed to find overlays.yaml file")
}
