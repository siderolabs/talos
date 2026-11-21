// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package lvfs

import (
	"bytes"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/siderolabs/talos/internal/pkg/firmware/cab"
	"github.com/siderolabs/talos/internal/pkg/firmware/jcat"
)

const (
	// metainfoSuffix names the AppStream metainfo files in a container; one
	// per component the container carries.
	metainfoSuffix = ".metainfo.xml"
	// signatureName is the jcat file signing the container's other files.
	signatureName = "firmware.jcat"
	// defaultPayloadName is the firmware image when the metainfo doesn't name it.
	defaultPayloadName = "firmware.bin"
)

// ExtractPayload returns the firmware image of a component from the
// container of one of its releases.
//
// The image is the file the catalog release names, or failing that the one
// named by the metainfo the container ships for the component. It is accepted
// only if its checksum matches the one the catalog release carries and its
// signature in the container's jcat file, like the metainfo's, verifies
// against roots.
func ExtractPayload(container []byte, componentID string, release *Release, roots *x509.CertPool) ([]byte, error) {
	archive, err := cab.Parse(container)
	if err != nil {
		return nil, fmt.Errorf("error parsing container: %w", err)
	}

	metainfoName, metainfo, component, err := findComponent(archive, componentID)
	if err != nil {
		return nil, err
	}

	imageName, expected := release.ContentChecksum()
	if expected == "" {
		return nil, errors.New("catalog release has no content checksum")
	}

	// the signed catalog names the image; the container's own metainfo is
	// only consulted when the catalog doesn't
	if imageName == "" {
		imageName = component.payloadName(release.Version)
	}

	image, err := archive.Open(imageName)
	if err != nil {
		return nil, fmt.Errorf("error reading image: %w", err)
	}

	if err = checkSHA256(image, expected); err != nil {
		return nil, fmt.Errorf("image %s: %w", imageName, err)
	}

	if err = verifySignatures(archive, roots, signedFile{imageName, image}, signedFile{metainfoName, metainfo}); err != nil {
		return nil, err
	}

	return image, nil
}

type signedFile struct {
	name string
	data []byte
}

// verifySignatures checks files against the container's jcat file.
func verifySignatures(archive *cab.Archive, roots *x509.CertPool, files ...signedFile) error {
	signature, err := archive.Open(signatureName)
	if err != nil {
		return fmt.Errorf("error reading signature: %w", err)
	}

	jf, err := jcat.Parse(bytes.NewReader(signature))
	if err != nil {
		return fmt.Errorf("error parsing %s: %w", signatureName, err)
	}

	for _, file := range files {
		item, ok := jf.Item(file.name)
		if !ok {
			return fmt.Errorf("%s is not signed", file.name)
		}

		if err = item.Verify(file.data, roots); err != nil {
			return fmt.Errorf("%s signature: %w", file.name, err)
		}
	}

	return nil
}

// findComponent locates the metainfo of a component in a container.
func findComponent(archive *cab.Archive, id string) (name string, metainfo []byte, component *Component, err error) {
	for _, f := range archive.Files() {
		if !strings.HasSuffix(f.Name, metainfoSuffix) {
			continue
		}

		data, err := archive.Open(f.Name)
		if err != nil {
			return "", nil, nil, fmt.Errorf("error reading %s: %w", f.Name, err)
		}

		var found *Component

		err = ParseComponents(bytes.NewReader(data), func(c *Component) error {
			if c.ID == id {
				found = c

				return io.EOF
			}

			return nil
		})
		if err != nil {
			return "", nil, nil, fmt.Errorf("error parsing %s: %w", f.Name, err)
		}

		if found != nil {
			return f.Name, data, found, nil
		}
	}

	return "", nil, nil, fmt.Errorf("component %q not found in container", id)
}

// ContentChecksum returns the SHA-256 of the release's firmware image and the
// image file name, when the checksum carries one.
func (r *Release) ContentChecksum() (filename, digest string) {
	for _, checksum := range r.Checksums {
		if strings.EqualFold(checksum.Type, "sha256") && checksum.Target == "content" {
			return checksum.Filename, strings.TrimSpace(checksum.Value)
		}
	}

	return "", ""
}

// payloadName returns the firmware image file named by the component's
// metainfo for the release of the given version, falling back to any release
// and then to the default name.
func (c *Component) payloadName(version string) string {
	for _, release := range c.Releases {
		if release.Version == version {
			if name := contentFilename(&release); name != "" {
				return name
			}
		}
	}

	for _, release := range c.Releases {
		if name := contentFilename(&release); name != "" {
			return name
		}
	}

	return defaultPayloadName
}

func contentFilename(r *Release) string {
	for _, checksum := range r.Checksums {
		if checksum.Target == "content" && checksum.Filename != "" {
			return checksum.Filename
		}
	}

	return ""
}
