// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package lvfs

import (
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Component is an AppStream firmware component from LVFS metadata.
type Component struct {
	ID       string `xml:"id"`
	Name     string `xml:"name"`
	Summary  string `xml:"summary"`
	Provides struct {
		Firmwares []ProvidedFirmware `xml:"firmware"`
	} `xml:"provides"`
	Releases []Release     `xml:"releases>release"`
	Custom   []CustomValue `xml:"custom>value"`
	Requires struct {
		Firmwares []FirmwareRequirement `xml:"firmware"`
	} `xml:"requires"`
}

// ProvidedFirmware is a firmware GUID provided by a component.
type ProvidedFirmware struct {
	Type  string `xml:"type,attr"`
	Value string `xml:",chardata"`
}

// FirmwareRequirement restricts which devices a component applies to.
type FirmwareRequirement struct {
	Compare string `xml:"compare,attr"`
	Version string `xml:"version,attr"`
	Value   string `xml:",chardata"`
}

// CustomValue is an LVFS custom metadata value.
type CustomValue struct {
	Key   string `xml:"key,attr"`
	Value string `xml:",chardata"`
}

// Release is a firmware release within a component.
type Release struct {
	Version     string     `xml:"version,attr"`
	Urgency     string     `xml:"urgency,attr"`
	Timestamp   int64      `xml:"timestamp,attr"`
	Locations   []string   `xml:"location"`
	Checksums   []Checksum `xml:"checksum"`
	Description struct {
		InnerXML string `xml:",innerxml"`
	} `xml:"description"`
	Artifacts []Artifact `xml:"artifacts>artifact"`
}

// Artifact is a downloadable file of a release.
type Artifact struct {
	Type      string     `xml:"type,attr"`
	Locations []string   `xml:"location"`
	Checksums []Checksum `xml:"checksum"`
}

// Checksum is a file checksum.
type Checksum struct {
	Type     string `xml:"type,attr"`
	Target   string `xml:"target,attr"`
	Filename string `xml:"filename,attr"`
	Value    string `xml:",chardata"`
}

// GUIDs returns the flashed-firmware GUIDs provided by the component.
func (c *Component) GUIDs() []string {
	var guids []string

	for _, fw := range c.Provides.Firmwares {
		if fw.Type == "" || fw.Type == "flashed" {
			guids = append(guids, strings.TrimSpace(fw.Value))
		}
	}

	return guids
}

// Protocol returns the LVFS update protocol ID of the component.
func (c *Component) Protocol() string {
	for _, v := range c.Custom {
		if v.Key == "LVFS::UpdateProtocol" {
			return strings.TrimSpace(v.Value)
		}
	}

	return ""
}

// LatestRelease returns the release with the highest timestamp.
func (c *Component) LatestRelease() (Release, bool) {
	if len(c.Releases) == 0 {
		return Release{}, false
	}

	latest := c.Releases[0]

	for _, release := range c.Releases[1:] {
		if release.Timestamp > latest.Timestamp {
			latest = release
		}
	}

	return latest, true
}

// Cab returns the download location and SHA-256 checksum of the release
// container, sourced from the same artifact so the pair can't be crossed.
//
// It prefers the first binary artifact that has both a location and a SHA-256
// checksum; otherwise it falls back to the release-level location paired with
// the release-level container SHA-256 checksum.
func (r *Release) Cab() (url, sha256 string) {
	for _, artifact := range r.Artifacts {
		if artifact.Type != "binary" {
			continue
		}

		u, s := artifactURLAndSHA256(&artifact)
		if u != "" && s != "" {
			return u, s
		}
	}

	if len(r.Locations) > 0 {
		url = r.Locations[0]
	}

	for _, checksum := range r.Checksums {
		if checksum.Type == "sha256" && checksum.Target == "container" {
			sha256 = strings.TrimSpace(checksum.Value)

			break
		}
	}

	return url, sha256
}

func artifactURLAndSHA256(a *Artifact) (url, sha256 string) {
	for _, location := range a.Locations {
		if location != "" {
			url = location

			break
		}
	}

	for _, checksum := range a.Checksums {
		if checksum.Type == "sha256" {
			sha256 = strings.TrimSpace(checksum.Value)

			break
		}
	}

	return url, sha256
}

// ParseComponents streams firmware components from AppStream XML, calling fn for each.
//
// Iteration stops early if fn returns an error; io.EOF from fn stops iteration without error.
func ParseComponents(r io.Reader, fn func(*Component) error) error {
	decoder := xml.NewDecoder(r)

	for {
		token, err := decoder.Token()
		if errors.Is(err, io.EOF) {
			return nil
		}

		if err != nil {
			return fmt.Errorf("error parsing AppStream XML: %w", err)
		}

		start, ok := token.(xml.StartElement)
		if !ok || start.Name.Local != "component" {
			continue
		}

		var component Component

		if err := decoder.DecodeElement(&component, &start); err != nil {
			return fmt.Errorf("error decoding AppStream component: %w", err)
		}

		if err := fn(&component); err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}

			return err
		}
	}
}
