// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package block

//docgen:jsonschema

import (
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"path"
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

// InlineDataVolumeConfigKind is a config document kind.
const InlineDataVolumeConfigKind = "InlineDataVolumeConfig"

func init() {
	registry.Register(InlineDataVolumeConfigKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &InlineDataVolumeConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.InlineDataVolumeConfig = &InlineDataVolumeConfigV1Alpha1{}
	_ config.ConflictingDocument    = &InlineDataVolumeConfigV1Alpha1{}
	_ config.NamedDocument          = &InlineDataVolumeConfigV1Alpha1{}
	_ config.Validator              = &InlineDataVolumeConfigV1Alpha1{}
)

// inlineDataVolumeDefaultFileMode is the mode of an inline data volume file which does not set one.
const inlineDataVolumeDefaultFileMode = 0o644

// InlineDataVolumeFileMode represents an inline data volume file's permissions.
type InlineDataVolumeFileMode fs.FileMode

// String converts file mode to octal string.
func (mode InlineDataVolumeFileMode) String() string {
	return "0o" + strconv.FormatUint(uint64(mode), 8)
}

// MarshalYAML encodes as an octal value.
func (mode InlineDataVolumeFileMode) MarshalYAML() (any, error) {
	return &yaml.Node{
		Kind:  yaml.ScalarNode,
		Tag:   "!!int",
		Value: mode.String(),
	}, nil
}

// InlineDataVolumeConfigV1Alpha1 is an inline data volume configuration document.
//
//	description: |
//	  Inline data volume is a set of files defined inline, similar to a Kubernetes ConfigMap.
//	  The files are kept in memory (`tmpfs`), which only Talos can write to.
//	  The name must not be taken by a user, existing or external volume.
//
//	  Changes to the files are applied in place:
//	  the files are replaced atomically as a whole, by swapping the `..data` symbolic link
//	  to a directory holding the new set of files.
//	  Each top-level entry of the volume is a symbolic link into `..data`.
//	examples:
//	  - value: exampleInlineDataVolumeConfigV1Alpha1()
//	alias: InlineDataVolumeConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/InlineDataVolumeConfig
type InlineDataVolumeConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Name of the volume.
	//
	//     Name might be between 1 and 34 characters long and can only contain:
	//     lowercase and uppercase ASCII letters, digits, and hyphens.
	//   schemaRequired: true
	MetaName string `yaml:"name"`
	//   description: |
	//     Files of the volume, keyed by the path relative to the volume root.
	//
	//     Paths may contain subdirectories, which are created automatically.
	//     Path components must not start with `..`, and must be at most 255 bytes long.
	//     Paths must be at most 1024 bytes long.
	//     There may be at most 256 files in at most 256 distinct directories,
	//     and their total size, each rounded up to 4 KiB, must not exceed 1 MiB.
	//   schemaRequired: true
	FilesConfig map[string]InlineDataVolumeFileV1Alpha1 `yaml:"files"`
}

// InlineDataVolumeFileV1Alpha1 describes a single file of an inline data volume.
type InlineDataVolumeFileV1Alpha1 struct {
	//   description: |
	//     The file's permissions in octal.
	//
	//     Defaults to `0o644`.
	//   schema:
	//     type: integer
	FileMode *InlineDataVolumeFileMode `yaml:"mode,omitempty"`
	//   description: |
	//     The contents of the file.
	FileContents string `yaml:"contents"`
}

// NewInlineDataVolumeConfigV1Alpha1 creates a new inline data volume config document.
func NewInlineDataVolumeConfigV1Alpha1() *InlineDataVolumeConfigV1Alpha1 {
	return &InlineDataVolumeConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       InlineDataVolumeConfigKind,
			MetaAPIVersion: "v1alpha1",
		},
	}
}

func exampleInlineDataVolumeConfigV1Alpha1() *InlineDataVolumeConfigV1Alpha1 {
	cfg := NewInlineDataVolumeConfigV1Alpha1()
	cfg.MetaName = "app-config"
	cfg.FilesConfig = map[string]InlineDataVolumeFileV1Alpha1{
		"app.yaml": {
			FileContents: "listen: :8080\n",
		},
		"conf.d/extra.ini": {
			FileMode:     new(InlineDataVolumeFileMode(0o600)),
			FileContents: "[section]\nkey=value\n",
		},
	}

	return cfg
}

// Name implements config.NamedDocument interface.
func (s *InlineDataVolumeConfigV1Alpha1) Name() string {
	return s.MetaName
}

// Clone implements config.Document interface.
func (s *InlineDataVolumeConfigV1Alpha1) Clone() config.Document {
	return s.DeepCopy()
}

// ConflictsWithKinds implements config.ConflictingDocument interface.
func (s *InlineDataVolumeConfigV1Alpha1) ConflictsWithKinds() []string {
	return conflictingVolumeKinds(InlineDataVolumeConfigKind)
}

// InlineDataVolumeConfigSignal is a signal for inline data volume config.
func (s *InlineDataVolumeConfigV1Alpha1) InlineDataVolumeConfigSignal() {}

// Files implements config.InlineDataVolumeConfig interface.
func (s *InlineDataVolumeConfigV1Alpha1) Files() []config.InlineDataVolumeFile {
	files := make([]config.InlineDataVolumeFile, 0, len(s.FilesConfig))

	for _, filePath := range slices.Sorted(maps.Keys(s.FilesConfig)) {
		file := s.FilesConfig[filePath]

		mode := fs.FileMode(inlineDataVolumeDefaultFileMode)
		if file.FileMode != nil {
			mode = fs.FileMode(*file.FileMode)
		}

		files = append(files, config.InlineDataVolumeFile{
			Path:     filePath,
			Mode:     mode,
			Contents: file.FileContents,
		})
	}

	return files
}

// Validate implements config.Validator interface.
func (s *InlineDataVolumeConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	validationErrors := validateVolumeName(s.MetaName, constants.InlineDataVolumePrefix)

	if len(s.FilesConfig) == 0 {
		validationErrors = errors.Join(validationErrors, errors.New("at least one file is required"))
	}

	paths := slices.Sorted(maps.Keys(s.FilesConfig))

	var totalSize int

	directories := map[string]struct{}{}

	for _, filePath := range paths {
		file := s.FilesConfig[filePath]

		validationErrors = errors.Join(validationErrors, s.validateFile(filePath, file))

		totalSize += inlineDataVolumeFileSize(file.FileContents)

		for dir := path.Dir(filePath); dir != "." && dir != "/" && dir != ""; dir = path.Dir(dir) {
			directories[dir] = struct{}{}
		}
	}

	if len(s.FilesConfig) > constants.InlineDataVolumeMaxFiles {
		validationErrors = errors.Join(validationErrors,
			fmt.Errorf("number of files %d exceeds the limit of %d", len(s.FilesConfig), constants.InlineDataVolumeMaxFiles))
	}

	if len(directories) > constants.InlineDataVolumeMaxDirectories {
		validationErrors = errors.Join(validationErrors,
			fmt.Errorf("number of directories %d exceeds the limit of %d", len(directories), constants.InlineDataVolumeMaxDirectories))
	}

	if totalSize > constants.InlineDataVolumeMaxContentsSize {
		validationErrors = errors.Join(validationErrors,
			fmt.Errorf("total size of the files %d (each rounded up to %d bytes) exceeds the limit of %d bytes",
				totalSize, constants.InlineDataVolumeBlockSize, constants.InlineDataVolumeMaxContentsSize))
	}

	return nil, validationErrors
}

// validateFile validates a single file of the inline data volume.
func (s *InlineDataVolumeConfigV1Alpha1) validateFile(filePath string, file InlineDataVolumeFileV1Alpha1) error {
	var validationErrors error

	if err := validateInlineDataVolumePath(filePath); err != nil {
		validationErrors = errors.Join(validationErrors, err)
	}

	for dir := path.Dir(filePath); dir != "." && dir != "/" && dir != ""; dir = path.Dir(dir) {
		if _, isFile := s.FilesConfig[dir]; isFile {
			validationErrors = errors.Join(validationErrors, fmt.Errorf("file %q conflicts with file %q: a path can't be both a file and a directory", dir, filePath))
		}
	}

	if file.FileMode != nil && fs.FileMode(*file.FileMode)&^fs.ModePerm != 0 {
		validationErrors = errors.Join(validationErrors, fmt.Errorf("file %q: mode %s may only contain permission bits", filePath, *file.FileMode))
	}

	return validationErrors
}

// inlineDataVolumeFileSize returns the file size rounded up to whole blocks of constants.InlineDataVolumeBlockSize.
//
// The block size is fixed rather than the page size of the system running the validation,
// so the same config is valid everywhere; the node sizes the tmpfs for its own page size.
func inlineDataVolumeFileSize(contents string) int {
	const blockSize = constants.InlineDataVolumeBlockSize

	return (len(contents) + blockSize - 1) / blockSize * blockSize
}

// validateInlineDataVolumePath validates a file path of an inline data volume.
//
// The path is slash-separated on any OS (the files are written on a Talos node),
// so it is checked with the path package rather than path/filepath.
func validateInlineDataVolumePath(filePath string) error {
	if filePath == "" {
		return errors.New("file path can't be empty")
	}

	if strings.ContainsRune(filePath, 0) {
		return fmt.Errorf("file path %q must not contain NUL bytes", filePath)
	}

	if len(filePath) > constants.InlineDataVolumeMaxPathLength {
		return fmt.Errorf("file path %q... is %d bytes long, exceeding the limit of %d bytes",
			filePath[:64], len(filePath), constants.InlineDataVolumeMaxPathLength)
	}

	if !isCleanRelativePath(filePath) {
		return fmt.Errorf("file path %q must be a clean relative path", filePath)
	}

	for component := range strings.SplitSeq(filePath, "/") {
		if strings.HasPrefix(component, "..") {
			return fmt.Errorf("file path %q: path components starting with \"..\" are reserved", filePath)
		}

		if len(component) > constants.InlineDataVolumeMaxNameLength {
			return fmt.Errorf("file path %q: path component is %d bytes long, exceeding the limit of %d bytes",
				filePath, len(component), constants.InlineDataVolumeMaxNameLength)
		}
	}

	return nil
}

// isCleanRelativePath reports whether a slash-separated path is relative, in its clean form,
// and doesn't refer to the root itself or escape it.
func isCleanRelativePath(filePath string) bool {
	if strings.HasPrefix(filePath, "/") || filePath == ".." || strings.HasPrefix(filePath, "../") ||
		filePath != path.Clean(filePath) {
		return false
	}

	for component := range strings.SplitSeq(filePath, "/") {
		if component == "" || component == "." {
			return false
		}
	}

	return true
}
