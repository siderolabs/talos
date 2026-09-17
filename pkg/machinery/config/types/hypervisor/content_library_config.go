// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package hypervisor

//docgen:jsonschema

import (
	"errors"
	"fmt"
	"regexp"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

// ContentLibraryConfigKind is a config document kind.
const ContentLibraryConfigKind = "ContentLibraryConfig"

// maxNameLength is the maximum length of a content library name.
// Way lower than NAME_MAX (255), to give us headroom for prefixes/suffixes down the reconciliation path.
const maxNameLength = 63

// validNamePattern matches the characters a content library name may contain.
var validNamePattern = regexp.MustCompile(`^[A-Za-z0-9-]+$`)

func init() {
	registry.Register(ContentLibraryConfigKind, func(version string) config.Document {
		switch version {
		case "v1alpha1":
			return &ContentLibraryConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.ContentLibraryConfig = &ContentLibraryConfigV1Alpha1{}
	_ config.NamedDocument        = &ContentLibraryConfigV1Alpha1{}
	_ config.Validator            = &ContentLibraryConfigV1Alpha1{}
)

// ContentLibraryConfigV1Alpha1 is a content library configuration document.
//
//	description: |
//	  ContentLibraryConfig declares a content library: a store for virtual machine images
//	  kept on a volume.
//
//	  The library is the root of the backing volume, so a library backed by the user volume
//	  `vm-images` keeps its images directly in `/var/mnt/vm-images`. A volume backs exactly
//	  one library: two libraries pointed at the same volume are rejected.
//
//	  Contents are managed over the API with `talosctl hv content-library`, never by editing
//	  the machine configuration. Status is reported via `ContentLibraryStatus`.
//	examples:
//	  - value: exampleContentLibraryConfigV1Alpha1()
//	alias: ContentLibraryConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/ContentLibraryConfig
type ContentLibraryConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Name of the content library.
	//
	//     Must be between 1 and 63 characters long, and can only contain ASCII letters,
	//     digits and hyphens. It is the ID used to address the library over the API.
	MetaName string `yaml:"name"`
	//   description: |
	//     Volume backing the content library.
	BackingConfig ContentLibraryBacking `yaml:"backing"`
}

// ContentLibraryBacking describes the storage backing a content library.
type ContentLibraryBacking struct {
	//   description: |
	//     Name of the volume storing the library's contents.
	//
	//     The name of a `UserVolumeConfig`, `ExistingVolumeConfig` or `ExternalVolumeConfig`
	//     document, as that document declares it, not the `u-`/`e-`/`x-` prefixed ID Talos
	//     derives from the kind and the name. Volume names are unique across those kinds, so
	//     the name resolves to one volume.
	//
	//     The volume is declared separately and is not provisioned by this document; the
	//     library becomes ready only once the volume is mounted. Uploads write into the
	//     library, so a volume declared read-only cannot back one.
	//   examples:
	//     - value: '"vm-images"'
	VolumeName string `yaml:"volume"`
}

// NewContentLibraryConfigV1Alpha1 creates a new content library config document.
func NewContentLibraryConfigV1Alpha1() *ContentLibraryConfigV1Alpha1 {
	return &ContentLibraryConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       ContentLibraryConfigKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleContentLibraryConfigV1Alpha1() *ContentLibraryConfigV1Alpha1 {
	cfg := NewContentLibraryConfigV1Alpha1()
	cfg.MetaName = "my-vm-images-1"
	cfg.BackingConfig = ContentLibraryBacking{
		VolumeName: "vm-images",
	}

	return cfg
}

// Name implements config.NamedDocument interface.
func (c *ContentLibraryConfigV1Alpha1) Name() string {
	return c.MetaName
}

// Clone implements config.Document interface.
func (c *ContentLibraryConfigV1Alpha1) Clone() config.Document {
	return c.DeepCopy()
}

// ContentLibraryConfigSignal is a signal for content library config.
func (c *ContentLibraryConfigV1Alpha1) ContentLibraryConfigSignal() {}

// BackingVolumeName implements config.ContentLibraryConfig interface.
func (c *ContentLibraryConfigV1Alpha1) BackingVolumeName() string {
	return c.BackingConfig.VolumeName
}

// Validate implements config.Validator interface.
func (c *ContentLibraryConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var validationErrors error

	validationErrors = errors.Join(validationErrors, c.ValidateName())
	validationErrors = errors.Join(validationErrors, c.ValidateBackingVolume())

	return nil, validationErrors
}

// ValidateName checks the content library name.
func (c *ContentLibraryConfigV1Alpha1) ValidateName() error {
	switch {
	case c.MetaName == "":
		return errors.New("name is required")
	case len(c.MetaName) > maxNameLength:
		return fmt.Errorf("name %q must be %d characters or fewer", c.MetaName, maxNameLength)
	case !validNamePattern.MatchString(c.MetaName):
		return fmt.Errorf("name %q: name can only contain ASCII letters, digits and hyphens", c.MetaName)
	}

	return nil
}

// ValidateBackingVolume checks the volume backing the content library.
//
// Only the name is checked here. Whether a volume of that name is declared, and which kind declares
// it, is resolved against the rest of the machine configuration.
//
// A name carrying what looks like an internal volume prefix is left alone: volumes may be declared
// under such a name, and the ID is derived from the document kind and the name, so `u-images`
// declared as a user volume is a volume of its own, addressed by ID as `u-u-images`.
func (c *ContentLibraryConfigV1Alpha1) ValidateBackingVolume() error {
	volumeName := c.BackingConfig.VolumeName

	switch {
	case volumeName == "":
		return errors.New("backing.volume is required")
	case !validNamePattern.MatchString(volumeName):
		return fmt.Errorf("backing.volume %q: volume name can only contain ASCII letters, digits and hyphens", volumeName)
	}

	return nil
}
