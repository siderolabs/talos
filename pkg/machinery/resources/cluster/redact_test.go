// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/resources/cluster"
)

// TestConfigRedaction covers the discovery configuration: the encryption key goes, the endpoints and
// the cluster ID stay, the same way the machine configuration redaction keeps cluster.id.
func TestConfigRedaction(t *testing.T) {
	t.Parallel()

	spec := cluster.ConfigSpec{
		RegistryKubernetesEnabled: true,
		ServiceEncryptionKey:      []byte("0123456789abcdef0123456789abcdef"),
		ServiceClusterID:          "cluster-id",
		ServiceEndpoints: []cluster.ServiceEndpoint{
			{Name: "default", Endpoint: "discovery.talos.dev:443"},
		},
	}

	assert.True(t, spec.HasSecrets(resource.Metadata{}))

	redacted := spec.DeepCopy()
	redacted.RedactSecrets(resource.Metadata{})

	// the key is a plain []byte, which renders as a list of numbers, so the placeholder would not read as
	// one: the key is cleared instead
	assert.Empty(t, redacted.ServiceEncryptionKey)
	assert.Equal(t, spec.ServiceClusterID, redacted.ServiceClusterID)
	assert.Equal(t, spec.ServiceEndpoints, redacted.ServiceEndpoints)
	assert.True(t, redacted.RegistryKubernetesEnabled)

	assert.NotEmpty(t, spec.ServiceEncryptionKey, "redacting the copy should not mutate the spec it was copied from")

	assert.False(t, (&cluster.ConfigSpec{RegistryKubernetesEnabled: true}).HasSecrets(resource.Metadata{}))
}
