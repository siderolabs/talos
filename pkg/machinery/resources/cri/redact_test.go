// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/cri"
)

// TestRegistriesConfigRedaction covers the registry credentials: what is configured for which
// registry stays readable, the credentials themselves do not.
func TestRegistriesConfigRedaction(t *testing.T) {
	t.Parallel()

	spec := cri.RegistriesConfigSpec{
		RegistryMirrors: map[string]*cri.RegistryMirrorConfig{
			"docker.io": {
				MirrorEndpoints: []cri.RegistryEndpointConfig{{EndpointEndpoint: "https://registry.local"}},
			},
		},
		RegistryAuths: map[string]*cri.RegistryAuthConfig{
			"registry.local": {
				RegistryUsername:      "talos",
				RegistryPassword:      "hunter2",
				RegistryAuth:          "dGFsb3M6aHVudGVyMg==",
				RegistryIdentityToken: "identity-token",
			},
		},
	}

	assert.True(t, spec.HasSecrets(resource.Metadata{}))

	redacted := spec.DeepCopy()
	redacted.RedactSecrets(resource.Metadata{})

	auth := redacted.RegistryAuths["registry.local"]

	// the username names the credential without authenticating anything, so it survives
	assert.Equal(t, "talos", auth.RegistryUsername)
	assert.Equal(t, constants.Redacted, auth.RegistryPassword)
	assert.Equal(t, constants.Redacted, auth.RegistryAuth)
	assert.Equal(t, constants.Redacted, auth.RegistryIdentityToken)

	// the mirrors are not secrets: which registry is pulled from where stays readable
	assert.Equal(t, spec.RegistryMirrors, redacted.RegistryMirrors)

	assert.Equal(t, "hunter2", spec.RegistryAuths["registry.local"].RegistryPassword,
		"redacting the copy should not mutate the spec it was copied from")
}

// TestRegistriesConfigRedactionEmpty makes sure a config with no credentials in it is not reported as
// carrying a secret: the annotation the API adds means something was taken away.
func TestRegistriesConfigRedactionEmpty(t *testing.T) {
	t.Parallel()

	spec := cri.RegistriesConfigSpec{
		RegistryMirrors: map[string]*cri.RegistryMirrorConfig{
			"docker.io": {
				MirrorEndpoints: []cri.RegistryEndpointConfig{{EndpointEndpoint: "https://registry.local"}},
			},
		},
		RegistryAuths: map[string]*cri.RegistryAuthConfig{
			"registry.local": {RegistryUsername: "talos"},
		},
	}

	assert.False(t, spec.HasSecrets(resource.Metadata{}))
}
