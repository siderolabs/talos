// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri_test

import (
	_ "embed"
	"strings"
	"testing"

	"github.com/opencontainers/runtime-spec/specs-go"
	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	coreconfig "github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/cri"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
)

//go:embed testdata/seccompprofileconfig.yaml
var expectedSeccompProfileConfigDocument []byte

func seccompProfile() map[string]any {
	return map[string]any{
		"defaultAction": "SCMP_ACT_ALLOW",
		"syscalls": []any{
			map[string]any{
				"action":   "SCMP_ACT_ERRNO",
				"errnoRet": 1,
				"names":    []any{"unshare", "setns"},
			},
		},
	}
}

func TestSeccompProfileConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := cri.NewSeccompProfileConfigV1Alpha1("no-ns.json")
	cfg.ProfileConfig.Object = seccompProfile()

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	assert.Equal(t, string(expectedSeccompProfileConfigDocument), string(marshaled))
}

func TestSeccompProfileConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedSeccompProfileConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &cri.SeccompProfileConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       cri.SeccompProfileConfigKind,
		},
		MetaName: "no-ns.json",
		ProfileConfig: meta.Unstructured{Object: map[string]any{
			"defaultAction": "SCMP_ACT_ALLOW",
			"syscalls": []any{
				map[string]any{
					"action":   "SCMP_ACT_ERRNO",
					"errnoRet": 1,
					"names":    []any{"unshare", "setns"},
				},
			},
		}},
	}, docs[0])

	profiles := provider.SeccompProfiles()
	require.Len(t, profiles, 1)
	assert.Equal(t, "no-ns.json", profiles[0].Name())
}

func TestSeccompProfileConfigMergeIsIdempotent(t *testing.T) {
	t.Parallel()

	document := cri.NewSeccompProfileConfigV1Alpha1("no-ns.json")
	document.ProfileConfig.Object = seccompProfile()

	currentContainer, err := container.New(document)
	require.NoError(t, err)

	var current coreconfig.Provider = currentContainer

	// The patch drops the syscalls list: the profile is replaced as a whole, not merged.
	patchDocument := cri.NewSeccompProfileConfigV1Alpha1("no-ns.json")
	patchDocument.ProfileConfig.Object = map[string]any{"defaultAction": "SCMP_ACT_LOG"}

	patchProvider, err := container.New(patchDocument)
	require.NoError(t, err)

	patch := configpatcher.NewStrategicMergePatch(patchProvider)
	current, err = configpatcher.StrategicMerge(current, patch)
	require.NoError(t, err)

	first, err := current.Bytes()
	require.NoError(t, err)

	current, err = configpatcher.StrategicMerge(current, patch)
	require.NoError(t, err)

	second, err := current.Bytes()
	require.NoError(t, err)

	assert.Equal(t, first, second)

	profiles := current.SeccompProfiles()
	require.Len(t, profiles, 1)
	assert.Equal(t, map[string]any{"defaultAction": "SCMP_ACT_LOG"}, profiles[0].Value())
}

func TestSeccompProfileConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name          string
		profileName   string
		profile       map[string]any
		expectedError string
	}{
		{
			name:        "valid",
			profileName: "no-ns.json",
			profile:     seccompProfile(),
		},
		{
			name:        "valid without extension",
			profileName: "My_Profile-1",
			profile:     map[string]any{"defaultAction": "SCMP_ACT_LOG"},
		},
		{
			name:          "empty name",
			profile:       seccompProfile(),
			expectedError: "name is required",
		},
		{
			name:          "path separator",
			profileName:   "../escape.json",
			profile:       seccompProfile(),
			expectedError: `name "../escape.json" may only contain ASCII letters, digits, '.', '_' and '-'`,
		},
		{
			name:          "dot dot",
			profileName:   "..",
			profile:       seccompProfile(),
			expectedError: `name ".." is not a valid file name`,
		},
		{
			name:          "too long",
			profileName:   strings.Repeat("a", 256),
			profile:       seccompProfile(),
			expectedError: "is too long, at most 255 characters are allowed",
		},
		{
			name:          "missing profile",
			profileName:   "empty.json",
			expectedError: "profile is required",
		},
		{
			name:          "missing default action",
			profileName:   "no-default.json",
			profile:       map[string]any{"syscalls": []any{}},
			expectedError: "seccomp profile: defaultAction is required",
		},
		{
			name:        "invalid syscalls",
			profileName: "broken.json",
			profile: map[string]any{
				"defaultAction": "SCMP_ACT_LOG",
				"syscalls":      "unshare",
			},
			expectedError: "failed to unmarshal seccomp profile",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := cri.NewSeccompProfileConfigV1Alpha1(test.profileName)
			cfg.ProfileConfig.Object = test.profile

			warnings, err := cfg.Validate(validationMode{})
			assert.Empty(t, warnings)

			if test.expectedError == "" {
				assert.NoError(t, err)
			} else {
				assert.ErrorContains(t, err, test.expectedError)
			}
		})
	}
}

func TestParseSeccompProfile(t *testing.T) {
	t.Parallel()

	profile, err := cri.ParseSeccompProfile(seccompProfile())
	require.NoError(t, err)

	errnoRet := uint(1)

	assert.Equal(t, &specs.LinuxSeccomp{
		DefaultAction: specs.ActAllow,
		Syscalls: []specs.LinuxSyscall{
			{
				Names:    []string{"unshare", "setns"},
				Action:   specs.ActErrno,
				ErrnoRet: &errnoRet,
			},
		},
	}, profile)
}

func TestSeccompProfileConfigV1Alpha1Conflict(t *testing.T) {
	t.Parallel()

	cfg := cri.NewSeccompProfileConfigV1Alpha1("audit.json")

	assert.NoError(t, cfg.V1Alpha1ConflictValidate(&v1alpha1.Config{}))
	assert.NoError(t, cfg.V1Alpha1ConflictValidate(&v1alpha1.Config{MachineConfig: &v1alpha1.MachineConfig{}}))

	legacy := &v1alpha1.Config{
		MachineConfig: &v1alpha1.MachineConfig{
			MachineSeccompProfiles: []*v1alpha1.MachineSeccompProfile{ //nolint:staticcheck // test deprecated compatibility
				{
					MachineSeccompProfileName:  "other.json",
					MachineSeccompProfileValue: meta.Unstructured{Object: map[string]any{"defaultAction": "SCMP_ACT_LOG"}},
				},
			},
		},
	}

	assert.NoError(t, cfg.V1Alpha1ConflictValidate(legacy))

	legacy.MachineConfig.MachineSeccompProfiles = append(legacy.MachineConfig.MachineSeccompProfiles, //nolint:staticcheck // test deprecated compatibility
		&v1alpha1.MachineSeccompProfile{ //nolint:staticcheck // test deprecated compatibility
			MachineSeccompProfileName:  "audit.json",
			MachineSeccompProfileValue: meta.Unstructured{Object: map[string]any{"defaultAction": "SCMP_ACT_LOG"}},
		},
	)

	assert.EqualError(t, cfg.V1Alpha1ConflictValidate(legacy), `seccomp profile "audit.json" is already declared in .machine.seccompProfiles`)
}

func TestSeccompProfilesMergeLegacy(t *testing.T) {
	t.Parallel()

	legacy := &v1alpha1.Config{
		MachineConfig: &v1alpha1.MachineConfig{
			MachineSeccompProfiles: []*v1alpha1.MachineSeccompProfile{ //nolint:staticcheck // test deprecated compatibility
				{
					MachineSeccompProfileName:  "legacy.json",
					MachineSeccompProfileValue: meta.Unstructured{Object: map[string]any{"defaultAction": "SCMP_ACT_LOG"}},
				},
			},
		},
	}

	document := cri.NewSeccompProfileConfigV1Alpha1("no-ns.json")
	document.ProfileConfig.Object = seccompProfile()

	cfg, err := container.New(legacy, document)
	require.NoError(t, err)

	assert.Equal(t,
		[]string{"legacy.json", "no-ns.json"},
		xslices.Map(cfg.SeccompProfiles(), config.SeccompProfile.Name),
	)

	duplicate := cri.NewSeccompProfileConfigV1Alpha1("legacy.json")
	duplicate.ProfileConfig.Object = seccompProfile()

	cfg, err = container.New(legacy, duplicate)
	require.NoError(t, err)

	_, err = cfg.ValidateAsClient(validationMode{})
	assert.ErrorContains(t, err, `seccomp profile "legacy.json" is already declared in .machine.seccompProfiles`)
}
