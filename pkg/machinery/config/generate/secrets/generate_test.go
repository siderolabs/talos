// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package secrets_test

import (
	stdx509 "crypto/x509"
	"testing"
	"time"

	"github.com/siderolabs/crypto/x509"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

func TestNewBundle(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name            string
		versionContract *config.VersionContract
	}{
		{
			name:            "v1.0",
			versionContract: config.TalosVersion1_0,
		},
		{
			name:            "v1.3",
			versionContract: config.TalosVersion1_3,
		},
		{
			name:            "v1.7",
			versionContract: config.TalosVersion1_7,
		},
		{
			name:            "current",
			versionContract: config.TalosVersionCurrent,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			_, err := secrets.NewBundle(secrets.NewFixedClock(time.Now()), test.versionContract)
			require.NoError(t, err)
		})
	}
}

func TestNewBundleFromConfig(t *testing.T) {
	t.Parallel()

	bundle, err := secrets.NewBundle(secrets.NewFixedClock(time.Now()), config.TalosVersionCurrent)
	require.NoError(t, err)

	osCA, err := x509.NewCertificateAuthorityFromCertificateAndKey(bundle.Certs.OS)
	require.NoError(t, err)

	assert.Equal(t, stdx509.Ed25519, osCA.Crt.PublicKeyAlgorithm, "expected Ed25519 signature algorithm")

	input, err := generate.NewInput("test", "https://localhost:6443", constants.DefaultKubernetesVersion, generate.WithSecretsBundle(bundle))
	require.NoError(t, err)

	cfg, err := input.Config(machine.TypeControlPlane)
	require.NoError(t, err)

	bundle2 := secrets.NewBundleFromConfig(bundle.Clock, cfg)

	assert.Equal(t, bundle, bundle2)
}
