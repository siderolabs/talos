// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package container_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/config"
	containercfg "github.com/siderolabs/talos/pkg/machinery/config/types/container"
	"github.com/siderolabs/talos/pkg/machinery/config/types/cri"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
)

// newSeccompContainerDoc builds a ContainerConfig document applying the given seccomp profile.
func newSeccompContainerDoc(name string, profileType config.ContainerSeccompProfile, localhostProfile string) *containercfg.ContainerConfigV1Alpha1 {
	doc := containercfg.NewContainerConfigV1Alpha1()
	doc.MetaName = name
	doc.ContainerImage = "docker.io/library/nginx:1.27"
	doc.SecurityConfig = &containercfg.ContainerSecurity{
		SecuritySeccompProfile: &containercfg.ContainerSeccomp{
			SeccompType:             profileType,
			SeccompLocalhostProfile: localhostProfile,
		},
	}

	return doc
}

func newSeccompProfileDoc(name string) *cri.SeccompProfileConfigV1Alpha1 {
	doc := cri.NewSeccompProfileConfigV1Alpha1(name)
	doc.ProfileConfig.Object = map[string]any{"defaultAction": "SCMP_ACT_LOG"}

	return doc
}

func TestContainerSeccompProfiles(t *testing.T) {
	t.Parallel()

	legacy := &v1alpha1.Config{
		ConfigVersion: "v1alpha1",
		MachineConfig: &v1alpha1.MachineConfig{
			MachineSeccompProfiles: []*v1alpha1.MachineSeccompProfile{ //nolint:staticcheck // test deprecated compatibility
				{
					MachineSeccompProfileName:  "legacy.json",
					MachineSeccompProfileValue: meta.Unstructured{Object: map[string]any{"defaultAction": "SCMP_ACT_LOG"}},
				},
			},
		},
	}

	for _, test := range []struct {
		name         string
		docs         []config.Document
		expectedErrs []string
	}{
		{
			name: "default profile",
			docs: []config.Document{
				newSeccompContainerDoc("app", config.ContainerSeccompProfileDefault, ""),
			},
		},
		{
			name: "declared profile",
			docs: []config.Document{
				newSeccompProfileDoc("audit.json"),
				newSeccompContainerDoc("app", config.ContainerSeccompProfileLocalhost, "audit.json"),
			},
		},
		{
			name: "profile shared by two containers",
			docs: []config.Document{
				newSeccompProfileDoc("audit.json"),
				newSeccompContainerDoc("app", config.ContainerSeccompProfileLocalhost, "audit.json"),
				newSeccompContainerDoc("web", config.ContainerSeccompProfileLocalhost, "audit.json"),
			},
		},
		{
			name: "legacy profile",
			docs: []config.Document{
				legacy,
				newSeccompContainerDoc("app", config.ContainerSeccompProfileLocalhost, "legacy.json"),
			},
		},
		{
			name: "profile not declared",
			docs: []config.Document{
				newSeccompProfileDoc("audit.json"),
				newSeccompContainerDoc("web", config.ContainerSeccompProfileLocalhost, "deny.json"),
				newSeccompContainerDoc("app", config.ContainerSeccompProfileLocalhost, "missing.json"),
			},
			expectedErrs: []string{
				`container "app": security.seccompProfile: no SeccompProfileConfig declares seccomp profile "missing.json"`,
				`container "web": security.seccompProfile: no SeccompProfileConfig declares seccomp profile "deny.json"`,
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := validateCrossDocs(t, test.docs...)

			if len(test.expectedErrs) == 0 {
				if err != nil {
					// The partial v1alpha1 document fails its own validation, which is not what
					// this test is about.
					assert.NotContains(t, err.Error(), "seccomp")
				}

				return
			}

			require.Error(t, err)

			for _, expectedErr := range test.expectedErrs {
				assert.ErrorContains(t, err, expectedErr)
			}
		})
	}
}
