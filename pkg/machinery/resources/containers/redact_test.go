// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containers_test

import (
	"testing"

	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
)

// TestContainerSpecRedaction covers the container command line and environment, which the machine
// configuration fills in verbatim: the names stay, the values go.
func TestContainerSpecRedaction(t *testing.T) {
	t.Parallel()

	spec := containers.ContainerSpecSpec{
		Image:       containers.ContainerImageSpec{Ref: "docker.io/library/nginx:1.29"},
		Entrypoint:  []string{"/docker-entrypoint.sh"},
		Args:        []string{"-g", "daemon off;", "--api-key=s3cret"},
		Environment: []string{"NGINX_PORT=8080", "DB_PASSWORD=hunter2", "NO_VALUE"},
	}

	assert.True(t, spec.HasSecrets(resource.Metadata{}))

	redacted := spec.DeepCopy()
	redacted.RedactSecrets(resource.Metadata{})

	assert.Equal(t, []string{
		"NGINX_PORT=" + constants.Redacted,
		"DB_PASSWORD=" + constants.Redacted,
		// nothing to take out of a bare name
		"NO_VALUE",
	}, redacted.Environment)
	assert.Equal(t, []string{"-g", "daemon off;", "--api-key=" + constants.Redacted}, redacted.Args)
	assert.Equal(t, spec.Entrypoint, redacted.Entrypoint)
	assert.Equal(t, spec.Image, redacted.Image)

	assert.Equal(t, "DB_PASSWORD=hunter2", spec.Environment[1],
		"redacting the copy should not mutate the spec it was copied from")
}

// TestContainerInstanceSpecRedaction covers the resolved instance, which carries the same values.
func TestContainerInstanceSpecRedaction(t *testing.T) {
	t.Parallel()

	spec := containers.ContainerInstanceSpecSpec{
		ContainerID: "web",
		Environment: []string{"DB_PASSWORD=hunter2"},
	}

	assert.True(t, spec.HasSecrets(resource.Metadata{}))

	redacted := spec.DeepCopy()
	redacted.RedactSecrets(resource.Metadata{})

	assert.Equal(t, []string{"DB_PASSWORD=" + constants.Redacted}, redacted.Environment)
	assert.Equal(t, "web", redacted.ContainerID)
}

// TestContainerSpecRedactionEmpty makes sure a container with nothing to take out is not reported as
// carrying a secret.
func TestContainerSpecRedactionEmpty(t *testing.T) {
	t.Parallel()

	spec := containers.ContainerSpecSpec{
		Image:      containers.ContainerImageSpec{Ref: "docker.io/library/nginx:1.29"},
		Entrypoint: []string{"/docker-entrypoint.sh"},
		Args:       []string{"-g", "daemon off;"},
	}

	assert.False(t, spec.HasSecrets(resource.Metadata{}))
	assert.False(t, (&containers.ContainerInstanceSpecSpec{}).HasSecrets(resource.Metadata{}))
}
