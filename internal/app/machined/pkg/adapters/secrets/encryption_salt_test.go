// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	secretsadapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/secrets"
	"github.com/siderolabs/talos/pkg/machinery/constants"
	"github.com/siderolabs/talos/pkg/machinery/resources/secrets"
)

func TestEncryptionSaltGenerate(t *testing.T) {
	t.Parallel()

	var spec1, spec2 secrets.EncryptionSaltSpec

	require.NoError(t, secretsadapter.EncryptionSalt(&spec1).Generate())
	require.NoError(t, secretsadapter.EncryptionSalt(&spec2).Generate())

	assert.NotEqual(t, spec1, spec2)

	assert.Len(t, spec1.DiskSalt, constants.DiskEncryptionSaltSize)
}
