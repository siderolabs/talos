// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri

import (
	"encoding/json"
	"errors"
	"fmt"
	"regexp"

	"github.com/opencontainers/runtime-spec/specs-go"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/registry"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/config/validation"
)

//docgen:jsonschema

// SeccompProfileConfigKind is the SeccompProfileConfig configuration document kind.
const SeccompProfileConfigKind = "SeccompProfileConfig"

// maxSeccompProfileNameLength is the maximum length of a seccomp profile name, which is used as a file name.
const maxSeccompProfileNameLength = 255

// validSeccompProfileNamePattern matches the characters a seccomp profile name may contain.
var validSeccompProfileNamePattern = regexp.MustCompile(`^[A-Za-z0-9._-]+$`)

func init() {
	registry.Register(SeccompProfileConfigKind, func(version string) config.Document {
		switch version {
		case "v1alpha1": //nolint:goconst
			return &SeccompProfileConfigV1Alpha1{}
		default:
			return nil
		}
	})
}

// Check interfaces.
var (
	_ config.SeccompProfileConfig         = &SeccompProfileConfigV1Alpha1{}
	_ config.Validator                    = &SeccompProfileConfigV1Alpha1{}
	_ container.V1Alpha1ConflictValidator = &SeccompProfileConfigV1Alpha1{}
)

// SeccompProfileConfigV1Alpha1 declares a named seccomp profile.
//
//	description: |
//	  SeccompProfileConfig declares a seccomp profile which can be referenced by name.
//
//	  Every profile is written to `/var/lib/kubelet/seccomp/profiles/<name>`, so Kubernetes pods
//	  can use it with `securityContext.seccompProfile.type: Localhost` and
//	  `localhostProfile: profiles/<name>`.
//
//	  This document replaces the deprecated `.machine.seccompProfiles` field.
//	examples:
//	  - value: exampleSeccompProfileConfigV1Alpha1()
//	alias: SeccompProfileConfig
//	schemaRoot: true
//	schemaMeta: v1alpha1/SeccompProfileConfig
type SeccompProfileConfigV1Alpha1 struct {
	meta.Meta `yaml:",inline"`

	//   description: |
	//     Name of the seccomp profile.
	//
	//     The name is used as the profile's file name, so it must be at most 255 characters long
	//     and may only contain ASCII letters, digits, `.`, `_` and `-`.
	//   examples:
	//     - value: '"audit.json"'
	//   schemaRequired: true
	MetaName string `yaml:"name"`
	//   description: |
	//     The seccomp profile, in the OCI runtime specification `LinuxSeccomp` format.
	//
	//     This is the same JSON format Kubernetes `Localhost` seccomp profiles use, expressed
	//     as YAML. `defaultAction` is required.
	//   schemaRequired: true
	//   schema:
	//     type: object
	ProfileConfig meta.Unstructured `yaml:"profile" merge:"replace"`
}

// NewSeccompProfileConfigV1Alpha1 creates a new SeccompProfileConfig document.
func NewSeccompProfileConfigV1Alpha1(name string) *SeccompProfileConfigV1Alpha1 {
	return &SeccompProfileConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       SeccompProfileConfigKind,
			MetaAPIVersion: "v1alpha1",
		},
		MetaName: name,
	}
}

func exampleSeccompProfileConfigV1Alpha1() *SeccompProfileConfigV1Alpha1 {
	cfg := NewSeccompProfileConfigV1Alpha1("audit.json")
	cfg.ProfileConfig.Object = map[string]any{
		"defaultAction": "SCMP_ACT_LOG",
	}

	return cfg
}

// Clone implements config.Document interface.
func (cfg *SeccompProfileConfigV1Alpha1) Clone() config.Document {
	return cfg.DeepCopy()
}

// Name implements config.NamedDocument interface.
func (cfg *SeccompProfileConfigV1Alpha1) Name() string {
	return cfg.MetaName
}

// Value implements config.SeccompProfile interface.
func (cfg *SeccompProfileConfigV1Alpha1) Value() map[string]any {
	return cfg.ProfileConfig.Object
}

// SeccompProfileConfigSignal implements config.SeccompProfileConfig interface.
func (cfg *SeccompProfileConfigV1Alpha1) SeccompProfileConfigSignal() {}

// V1Alpha1ConflictValidate implements container.V1Alpha1ConflictValidator interface.
func (cfg *SeccompProfileConfigV1Alpha1) V1Alpha1ConflictValidate(v1alpha1Cfg *v1alpha1.Config) error {
	for _, profile := range v1alpha1Cfg.SeccompProfiles() {
		if profile.Name() == cfg.MetaName {
			return fmt.Errorf("seccomp profile %q is already declared in .machine.seccompProfiles", cfg.MetaName)
		}
	}

	return nil
}

// Validate implements config.Validator interface.
func (cfg *SeccompProfileConfigV1Alpha1) Validate(validation.RuntimeMode, ...validation.Option) ([]string, error) {
	var validationErrors error

	switch {
	case cfg.MetaName == "":
		validationErrors = errors.Join(validationErrors, errors.New("name is required"))
	case len(cfg.MetaName) > maxSeccompProfileNameLength:
		validationErrors = errors.Join(validationErrors,
			fmt.Errorf("name %q is too long, at most %d characters are allowed", cfg.MetaName, maxSeccompProfileNameLength))
	case cfg.MetaName == "." || cfg.MetaName == "..":
		validationErrors = errors.Join(validationErrors, fmt.Errorf("name %q is not a valid file name", cfg.MetaName))
	case !validSeccompProfileNamePattern.MatchString(cfg.MetaName):
		validationErrors = errors.Join(validationErrors,
			fmt.Errorf("name %q may only contain ASCII letters, digits, '.', '_' and '-'", cfg.MetaName))
	}

	if len(cfg.ProfileConfig.Object) == 0 {
		return nil, errors.Join(validationErrors, errors.New("profile is required"))
	}

	if _, err := ParseSeccompProfile(cfg.ProfileConfig.Object); err != nil {
		validationErrors = errors.Join(validationErrors, err)
	}

	return nil, validationErrors
}

// ParseSeccompProfile converts a seccomp profile from its configuration form into the OCI runtime-spec type.
func ParseSeccompProfile(profile map[string]any) (*specs.LinuxSeccomp, error) {
	jsonProfile, err := json.Marshal(profile)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal seccomp profile: %w", err)
	}

	var seccomp specs.LinuxSeccomp

	if err = json.Unmarshal(jsonProfile, &seccomp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal seccomp profile: %w", err)
	}

	if seccomp.DefaultAction == "" {
		return nil, errors.New("seccomp profile: defaultAction is required")
	}

	return &seccomp, nil
}
