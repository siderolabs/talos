// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package rotatepatcher_test

import (
	"testing"
	"time"

	"github.com/siderolabs/crypto/x509"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/machinery/config/generate/rotatepatcher"
	"github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
)

func newEtcdCA(t *testing.T) *x509.PEMEncodedCertificateAndKey {
	t.Helper()

	ca, err := x509.NewSelfSignedCertificateAuthority()
	require.NoError(t, err)

	return &x509.PEMEncodedCertificateAndKey{
		Crt: ca.CrtPEM,
		Key: ca.KeyPEM,
	}
}

func etcdConfig(t *testing.T, bundle *secrets.Bundle, versionContract *config.VersionContract) config.Provider {
	t.Helper()

	in, err := generate.NewInput(
		"test", "https://127.0.0.1:6443", "1.34.0",
		generate.WithSecretsBundle(bundle),
		generate.WithVersionContract(versionContract),
	)
	require.NoError(t, err)

	cfg, err := in.Config(machine.TypeControlPlane)
	require.NoError(t, err)

	return cfg
}

func applyPatches(t *testing.T, cfg config.Provider, patches ...rotatepatcher.PatcherFunc) config.Provider {
	t.Helper()

	for _, patch := range patches {
		var err error

		cfg, err = patch(cfg)
		require.NoError(t, err)
	}

	// the patched config should be accepted when applied to the machine
	_, err := cfg.ValidateAsClient(validationMode{})
	require.NoError(t, err)

	return cfg
}

type validationMode struct{}

func (validationMode) String() string        { return "test" }
func (validationMode) RequiresInstall() bool { return false }
func (validationMode) InContainer() bool     { return false }

func acceptedCAs(crts ...[]byte) []*x509.PEMEncodedCertificate {
	result := make([]*x509.PEMEncodedCertificate, 0, len(crts))

	for _, crt := range crts {
		result = append(result, &x509.PEMEncodedCertificate{Crt: crt})
	}

	return result
}

func TestEtcdCARotatePatchers(t *testing.T) {
	t.Parallel()

	bundle, err := secrets.NewBundle(secrets.NewFixedClock(time.Now()), config.TalosVersionCurrent)
	require.NoError(t, err)

	currentCA := bundle.Certs.Etcd
	newCA := newEtcdCA(t)
	extraCA := newEtcdCA(t)

	cfg := etcdConfig(t, bundle, config.TalosVersionCurrent)

	// base state
	assert.Equal(t, currentCA, cfg.EtcdCAConfig().IssuingCA())
	assert.Equal(t, acceptedCAs(currentCA.Crt), cfg.EtcdCAConfig().AcceptedCAs())

	t.Run("AddAcceptedCA", func(t *testing.T) {
		t.Parallel()

		// adding is idempotent, and the issuing CA is never added as accepted
		patched := applyPatches(
			t, cfg,
			rotatepatcher.EtcdAddAcceptedCA(newCA.Crt),
			rotatepatcher.EtcdAddAcceptedCA(newCA.Crt),
			rotatepatcher.EtcdAddAcceptedCA(currentCA.Crt),
		)

		assert.Equal(t, currentCA, patched.EtcdCAConfig().IssuingCA())
		assert.Equal(t, acceptedCAs(currentCA.Crt, newCA.Crt), patched.EtcdCAConfig().AcceptedCAs())

		// the original config is not modified
		assert.Equal(t, acceptedCAs(currentCA.Crt), cfg.EtcdCAConfig().AcceptedCAs())
	})

	t.Run("DeleteAcceptedCA", func(t *testing.T) {
		t.Parallel()

		patched := applyPatches(
			t, cfg,
			rotatepatcher.EtcdAddAcceptedCA(newCA.Crt),
			rotatepatcher.EtcdAddAcceptedCA(extraCA.Crt),
			rotatepatcher.EtcdDeleteAcceptedCA(newCA.Crt),
			// the issuing CA can't be deleted
			rotatepatcher.EtcdDeleteAcceptedCA(currentCA.Crt),
		)

		assert.Equal(t, currentCA, patched.EtcdCAConfig().IssuingCA())
		assert.Equal(t, acceptedCAs(currentCA.Crt, extraCA.Crt), patched.EtcdCAConfig().AcceptedCAs())
	})

	t.Run("SetCA", func(t *testing.T) {
		t.Parallel()

		// the previous issuing CA is kept as accepted, and setting the CA is idempotent
		patched := applyPatches(
			t, cfg,
			rotatepatcher.EtcdAddAcceptedCA(newCA.Crt),
			rotatepatcher.EtcdSetCA(newCA),
			rotatepatcher.EtcdSetCA(newCA),
		)

		assert.Equal(t, newCA, patched.EtcdCAConfig().IssuingCA())
		assert.Equal(t, acceptedCAs(newCA.Crt, currentCA.Crt), patched.EtcdCAConfig().AcceptedCAs())
	})

	t.Run("Rotate", func(t *testing.T) {
		t.Parallel()

		// mirrors the phases of pkg/rotate/pki/etcd.rotator: addNewCAAccepted, swapCAs, dropOldCAs
		patched := applyPatches(
			t, cfg,
			rotatepatcher.EtcdAddAcceptedCA(extraCA.Crt),
			// addNewCAAccepted
			rotatepatcher.EtcdAddAcceptedCA(newCA.Crt),
			// swapCAs
			rotatepatcher.EtcdAddAcceptedCA(currentCA.Crt),
			rotatepatcher.EtcdSetCA(newCA),
			// dropOldCAs
			rotatepatcher.EtcdDeleteAcceptedCA(currentCA.Crt),
		)

		// the unrelated accepted CA is kept
		assert.Equal(t, newCA, patched.EtcdCAConfig().IssuingCA())
		assert.Equal(t, acceptedCAs(newCA.Crt, extraCA.Crt), patched.EtcdCAConfig().AcceptedCAs())
	})
}

func TestEtcdCARotateLegacyConfig(t *testing.T) {
	t.Parallel()

	bundle, err := secrets.NewBundle(secrets.NewFixedClock(time.Now()), config.TalosVersion1_14)
	require.NoError(t, err)

	cfg := etcdConfig(t, bundle, config.TalosVersion1_14)
	newCA := newEtcdCA(t)

	for _, patch := range []rotatepatcher.PatcherFunc{
		rotatepatcher.EtcdAddAcceptedCA(newCA.Crt),
		rotatepatcher.EtcdDeleteAcceptedCA(newCA.Crt),
		rotatepatcher.EtcdSetCA(newCA),
	} {
		_, err = patch(cfg)
		require.ErrorIs(t, err, rotatepatcher.ErrEtcdLegacyConfig)
	}
}
