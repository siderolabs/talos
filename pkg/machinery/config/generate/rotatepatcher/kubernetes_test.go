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

// TestK8sCARotatePatchers verifies that the rotatepatcher methods produce the same result via the
// config.K8sAPIServerCAConfig interface regardless of the machine config version contract.
//
// Talos 1.13 stores the Kubernetes API server CA in the legacy .cluster.ca/.cluster.acceptedCAs
// fields of the v1alpha1 config, while Talos 1.14 stores it in the multi-doc KubeAPIServerCAConfig
// document. The rotatepatcher methods must hide this difference from the caller.
func TestK8sCARotatePatchers(t *testing.T) {
	t.Parallel()

	// use a single shared secrets bundle so that both version contracts are generated with the
	// exact same Kubernetes CA, making the results directly comparable.
	bundle, err := secrets.NewBundle(secrets.NewFixedClock(time.Now()), config.TalosVersion1_14)
	require.NoError(t, err)

	k8sCA := bundle.Certs.K8s

	extraCA := &x509.PEMEncodedCertificate{
		Crt: []byte("-----BEGIN CERTIFICATE-----\nEXTRA-ACCEPTED-CA\n-----END CERTIFICATE-----"),
	}

	newCA := &x509.PEMEncodedCertificateAndKey{
		Crt: []byte("-----BEGIN CERTIFICATE-----\nNEW-ISSUING-CA\n-----END CERTIFICATE-----"),
		Key: []byte("-----BEGIN EC PRIVATE KEY-----\nNEW-ISSUING-KEY\n-----END EC PRIVATE KEY-----"),
	}

	for _, test := range []struct {
		name        string
		machineType machine.Type

		patch func(config.Provider) (config.Provider, error)

		expectIssuingCA   *x509.PEMEncodedCertificateAndKey
		expectAcceptedCAs []*x509.PEMEncodedCertificate
	}{
		{
			name:        "AddAcceptedCA controlplane",
			machineType: machine.TypeControlPlane,

			patch: rotatepatcher.K8sAddAcceptedCA(extraCA.Crt),

			// the controlplane keeps its issuing CA (cert + key), and the issuing CA cert is
			// always reported as the first accepted CA.
			expectIssuingCA: &x509.PEMEncodedCertificateAndKey{Crt: k8sCA.Crt, Key: k8sCA.Key},
			expectAcceptedCAs: []*x509.PEMEncodedCertificate{
				{Crt: k8sCA.Crt},
				{Crt: extraCA.Crt},
			},
		},
		{
			name:        "AddAcceptedCA worker",
			machineType: machine.TypeWorker,

			patch: rotatepatcher.K8sAddAcceptedCA(extraCA.Crt),

			// the worker has no issuing CA, only accepted CAs.
			expectIssuingCA: nil,
			expectAcceptedCAs: []*x509.PEMEncodedCertificate{
				{Crt: k8sCA.Crt},
				{Crt: extraCA.Crt},
			},
		},
		{
			name:        "DeleteAcceptedCA controlplane",
			machineType: machine.TypeControlPlane,

			// add and then remove the same accepted CA: the config should be back to the base state.
			patch: func(in config.Provider) (config.Provider, error) {
				added, err := rotatepatcher.K8sAddAcceptedCA(extraCA.Crt)(in)
				if err != nil {
					return nil, err
				}

				return rotatepatcher.K8sDeleteAcceptedCA(extraCA.Crt)(added)
			},

			expectIssuingCA: &x509.PEMEncodedCertificateAndKey{Crt: k8sCA.Crt, Key: k8sCA.Key},
			expectAcceptedCAs: []*x509.PEMEncodedCertificate{
				{Crt: k8sCA.Crt},
			},
		},
		{
			name:        "DeleteAcceptedCA worker",
			machineType: machine.TypeWorker,

			patch: func(in config.Provider) (config.Provider, error) {
				added, err := rotatepatcher.K8sAddAcceptedCA(extraCA.Crt)(in)
				if err != nil {
					return nil, err
				}

				return rotatepatcher.K8sDeleteAcceptedCA(extraCA.Crt)(added)
			},

			expectIssuingCA: nil,
			expectAcceptedCAs: []*x509.PEMEncodedCertificate{
				{Crt: k8sCA.Crt},
			},
		},
		{
			name:        "SetCA controlplane",
			machineType: machine.TypeControlPlane,

			patch: rotatepatcher.K8sSetCA(newCA),

			// on the controlplane, SetCA replaces the issuing CA with the new key pair.
			expectIssuingCA: &x509.PEMEncodedCertificateAndKey{Crt: newCA.Crt, Key: newCA.Key},
			expectAcceptedCAs: []*x509.PEMEncodedCertificate{
				{Crt: newCA.Crt},
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			for _, versionContract := range []*config.VersionContract{
				config.TalosVersion1_13,
				config.TalosVersion1_14,
			} {
				t.Run(versionContract.String(), func(t *testing.T) {
					t.Parallel()

					in, err := generate.NewInput(
						"test", "https://127.0.0.1:6443", "1.34.0",
						generate.WithSecretsBundle(bundle),
						generate.WithVersionContract(versionContract),
					)
					require.NoError(t, err)

					cfg, err := in.Config(test.machineType)
					require.NoError(t, err)

					patched, err := test.patch(cfg)
					require.NoError(t, err)

					caConfig := patched.K8sAPIServerCAConfig()
					require.NotNil(t, caConfig)

					assert.Equal(t, test.expectIssuingCA, caConfig.IssuingCA())
					assert.Equal(t, test.expectAcceptedCAs, caConfig.AcceptedCAs())
				})
			}
		})
	}
}

// TestK8sCARotateComposed mirrors the composed CA rotation performed by pkg/rotate/pki/kubernetes:
// add the new CA as accepted, swap the issuing/accepted CAs, then drop the old CA.
//
// Unlike an isolated K8sSetCA call on a worker (whose storage differs between versions), the full
// composed rotation converges to the exact same result via the K8sAPIServerCAConfig interface
// regardless of the machine config version contract.
func TestK8sCARotateComposed(t *testing.T) {
	t.Parallel()

	// use a single shared secrets bundle so both version contracts start from the exact same CA.
	bundle, err := secrets.NewBundle(secrets.NewFixedClock(time.Now()), config.TalosVersion1_14)
	require.NoError(t, err)

	// the CA the cluster was generated with, i.e. the "current" CA in the rotator.
	currentCA := bundle.Certs.K8s.Crt

	// a freshly generated CA to rotate to.
	newCAAuthority, err := x509.NewSelfSignedCertificateAuthority()
	require.NoError(t, err)

	newCA := &x509.PEMEncodedCertificateAndKey{
		Crt: newCAAuthority.CrtPEM,
		Key: newCAAuthority.KeyPEM,
	}

	// rotate mirrors the phases of pkg/rotate/pki/kubernetes.rotator: addNewCAAccepted, swapCAs, dropOldCA.
	rotate := func(in config.Provider) (config.Provider, error) {
		// addNewCAAccepted
		out, err := rotatepatcher.K8sAddAcceptedCA(newCA.Crt)(in)
		if err != nil {
			return nil, err
		}

		// swapCAs
		if out, err = rotatepatcher.K8sAddAcceptedCA(currentCA)(out); err != nil {
			return nil, err
		}

		if out, err = rotatepatcher.K8sDeleteAcceptedCA(newCA.Crt)(out); err != nil {
			return nil, err
		}

		if out, err = rotatepatcher.K8sSetCA(newCA)(out); err != nil {
			return nil, err
		}

		// dropOldCA
		return rotatepatcher.K8sDeleteAcceptedCA(currentCA)(out)
	}

	for _, test := range []struct {
		name        string
		machineType machine.Type

		expectIssuingCA   *x509.PEMEncodedCertificateAndKey
		expectAcceptedCAs []*x509.PEMEncodedCertificate
	}{
		{
			name:        "controlplane",
			machineType: machine.TypeControlPlane,

			// the controlplane ends up issuing with the new CA, which is the only accepted CA.
			expectIssuingCA:   &x509.PEMEncodedCertificateAndKey{Crt: newCA.Crt, Key: newCA.Key},
			expectAcceptedCAs: []*x509.PEMEncodedCertificate{{Crt: newCA.Crt}},
		},
		{
			name:        "worker",
			machineType: machine.TypeWorker,

			// the worker never gets the issuing key, and ends up trusting only the new CA.
			expectIssuingCA:   nil,
			expectAcceptedCAs: []*x509.PEMEncodedCertificate{{Crt: newCA.Crt}},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			for _, versionContract := range []*config.VersionContract{
				config.TalosVersion1_13,
				config.TalosVersion1_14,
			} {
				t.Run(versionContract.String(), func(t *testing.T) {
					t.Parallel()

					in, err := generate.NewInput(
						"test", "https://127.0.0.1:6443", "1.34.0",
						generate.WithSecretsBundle(bundle),
						generate.WithVersionContract(versionContract),
					)
					require.NoError(t, err)

					cfg, err := in.Config(test.machineType)
					require.NoError(t, err)

					rotated, err := rotate(cfg)
					require.NoError(t, err)

					caConfig := rotated.K8sAPIServerCAConfig()
					require.NotNil(t, caConfig)

					assert.Equal(t, test.expectIssuingCA, caConfig.IssuingCA())
					assert.Equal(t, test.expectAcceptedCAs, caConfig.AcceptedCAs())
				})
			}
		})
	}
}
