// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime_test

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/runtime"
)

// TestEnvironmentRedactSecrets covers the environment of the machined process, which carries the
// variables of the machine configuration: the names stay, the values go.
func TestEnvironmentRedactSecrets(t *testing.T) {
	t.Parallel()

	env := runtime.NewEnvironment("machined")
	md := *env.Metadata()

	assert.False(t, env.TypedSpec().HasSecrets(md))

	env.TypedSpec().Variables = []string{
		"AWS_SECRET_ACCESS_KEY=wJalrXU",
		"GRPC_GO_LOG_SEVERITY_LEVEL=info",
		"PATH=/sbin:/bin",
		// os.Environ() never yields one of these, but a redactor may not depend on that
		"NO_VALUE",
	}

	assert.True(t, env.TypedSpec().HasSecrets(md))

	env.TypedSpec().RedactSecrets(md)

	assert.Equal(t, []string{
		"AWS_SECRET_ACCESS_KEY=" + constants.Redacted,
		"GRPC_GO_LOG_SEVERITY_LEVEL=" + constants.Redacted,
		"PATH=" + constants.Redacted,
		"NO_VALUE",
	}, env.TypedSpec().Variables)
}
