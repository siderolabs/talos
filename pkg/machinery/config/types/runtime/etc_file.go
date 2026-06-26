// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

//docgen:jsonschema

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"go.yaml.in/yaml/v4"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// EtcFileConfigKind is a user /etc file config document kind.
const EtcFileConfigKind = "EtcFileConfig"

func init() {
	registry.Register(EtcFileConfigKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &EtcFileConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.EtcFileConfig = &EtcFileConfigV1Alpha1{}
	_ config.NamedDocument = &EtcFileConfigV1Alpha1{}
	_ config.Validator     = &EtcFileConfigV1Alpha1{}
)

// EtcFileMode represents a user /etc file's permissions.
type EtcFileMode os.FileMode

// String converts file mode to octal string.
func (mode EtcFileMode) String() string {
	return "0o" + strconv.FormatUint(uint64(mode), 8)
}

// MarshalYAML encodes as an octal value.
func (mode EtcFileMode) MarshalYAML() (any, error) {
	return &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!int",
		Value: mode.String(),
	}, nil
}

// EtcFileConfigV1Alpha1 configures a user-managed file under /etc.
//
//	examples:
//	  - value: exampleEtcFileConfigV1Alpha1()
//	alias: EtcFileConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/EtcFileConfig
type EtcFileConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Path of the file relative to `/etc`.
	//   schemaRequired: true
	MetaName string `yaml:"name"`
	//   description: |
	//     The file's permissions in octal.
	//   schema:
	//     type: integer
	FileMode EtcFileMode `yaml:"mode"`
	//   description: |
	//     The contents of the file.
	Contents string `yaml:"contents"`
}

// NewEtcFileConfigV1Alpha1 creates a new EtcFileConfig config document.
func NewEtcFileConfigV1Alpha1(name string) *EtcFileConfigV1Alpha1 {
	return &EtcFileConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       EtcFileConfigKind,
			MetaAPIVersion: "v1alpha1",
		},
		MetaName: name,
		FileMode: 0o644,
	}
}

func exampleEtcFileConfigV1Alpha1() *EtcFileConfigV1Alpha1 {
	cfg := NewEtcFileConfigV1Alpha1("nfsmount.conf")
	cfg.Contents = "[NFSMount_Global_Options]\n"

	return cfg
}

// Clone implements config.Document interface.
func (cfg *EtcFileConfigV1Alpha1) Clone() config.Document {
	return cfg.DeepCopy()
}

// Name implements config.NamedDocument interface.
func (cfg *EtcFileConfigV1Alpha1) Name() string {
	return cfg.MetaName
}

// Content implements config.EtcFileConfig interface.
func (cfg *EtcFileConfigV1Alpha1) Content() string {
	return cfg.Contents
}

// Mode implements config.EtcFileConfig interface.
func (cfg *EtcFileConfigV1Alpha1) Mode() fs.FileMode {
	if cfg.FileMode == 0 {
		return 0o644
	}

	return fs.FileMode(cfg.FileMode)
}

// Validate implements config.Validator interface.
func (cfg *EtcFileConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	if cfg.MetaName == "" {
		return nil, errors.New("user etc file name cannot be empty")
	}

	if err := validateEtcFilePath(cfg.MetaName); err != nil {
		return nil, err
	}

	return nil, nil
}

var managedEtcFiles = []string{
	"resolv.conf",
	"hosts",
	"machine-id",
	"extensions.yaml",
	"localtime",
	"os-release",
	"xattr.conf",
	constants.CRIConfig,
	constants.CRICustomizationConfigPart,
	constants.CRIBaseRuntimeSpec,
	constants.DefaultTrustedRelativeCAFile,
	"iscsi/initiatorname.iscsi",
	"nvme/hostid",
	"nvme/hostnqn",
}

var managedEtcPrefixes = []string{
	"cni/",
	"kubernetes/",
	"cri/",
	"apparmor/",
	"apparmor.d/",
	"ca-certificates/",
	"lvm/",
	"pki/",
	"selinux/",
	"ssl/",
}

func validateEtcFilePath(path string) error {
	if !filepath.IsLocal(path) || strings.HasPrefix(path, ".") {
		return fmt.Errorf("user etc file %q must be a local path", path)
	}

	if slices.Contains(managedEtcFiles, path) {
		return fmt.Errorf("user etc file %q is managed by Talos", path)
	}

	for _, prefix := range managedEtcPrefixes {
		if path == strings.TrimSuffix(prefix, "/") || strings.HasPrefix(path, prefix) {
			return fmt.Errorf("user etc file %q is managed by Talos", path)
		}
	}

	return nil
}
