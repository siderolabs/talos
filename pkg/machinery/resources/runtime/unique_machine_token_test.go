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

// TestUniqueMachineTokenRedactSecrets covers the token which is the resource form of the
// UniqueMachineToken META key.
func TestUniqueMachineTokenRedactSecrets(t *testing.T) {
	t.Parallel()

	token := runtime.NewUniqueMachineToken()
	md := *token.Metadata()

	assert.False(t, token.TypedSpec().HasSecrets(md), "a machine with no token assigned has nothing to redact")

	token.TypedSpec().Token = "v1:E4t5..."

	assert.True(t, token.TypedSpec().HasSecrets(md))

	token.TypedSpec().RedactSecrets(md)

	assert.Equal(t, constants.Redacted, token.TypedSpec().Token)
}
