// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package v1alpha1_test

import (
	"testing"

	"github.com/siderolabs/crypto/x509"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

func TestEtcdBridge(t *testing.T) {
	t.Parallel()

	t.Run("no cluster config", func(t *testing.T) {
		t.Parallel()

		cfg := &v1alpha1.Config{}

		assert.Nil(t, cfg.EtcdCAConfig())
		assert.Nil(t, cfg.EtcdMemberConfig())
	})

	t.Run("no etcd config", func(t *testing.T) {
		t.Parallel()

		cfg := &v1alpha1.Config{
			ClusterConfig: &v1alpha1.ClusterConfig{},
		}

		assert.Nil(t, cfg.EtcdCAConfig())
		assert.Nil(t, cfg.EtcdMemberConfig())
	})

	t.Run("etcd config without CA", func(t *testing.T) {
		t.Parallel()

		cfg := &v1alpha1.Config{
			ClusterConfig: &v1alpha1.ClusterConfig{
				EtcdConfig: &v1alpha1.EtcdConfig{}, //nolint:staticcheck // testing deprecated field
			},
		}

		// without the CA etcd can't run, so neither config is reported
		assert.Nil(t, cfg.EtcdCAConfig())
		assert.Nil(t, cfg.EtcdMemberConfig())
	})

	t.Run("etcd config with CA only", func(t *testing.T) {
		t.Parallel()

		cfg := &v1alpha1.Config{
			ClusterConfig: &v1alpha1.ClusterConfig{
				EtcdConfig: &v1alpha1.EtcdConfig{ //nolint:staticcheck // testing deprecated field
					RootCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("cert"),
						Key: []byte("key"),
					},
				},
			},
		}

		assert.NotNil(t, cfg.EtcdCAConfig())

		member := cfg.EtcdMemberConfig()
		require.NotNil(t, member)

		assert.Equal(t, constants.EtcdImage+":"+constants.DefaultEtcdVersion, member.Image())
		assert.Empty(t, member.ExtraArgs())
		assert.Empty(t, member.AdvertisedSubnets())
		assert.Empty(t, member.ListenSubnets())
	})

	t.Run("full etcd config", func(t *testing.T) {
		t.Parallel()

		ca := &x509.PEMEncodedCertificateAndKey{
			Crt: []byte("cert"),
			Key: []byte("key"),
		}

		cfg := &v1alpha1.Config{
			ClusterConfig: &v1alpha1.ClusterConfig{
				EtcdConfig: &v1alpha1.EtcdConfig{ //nolint:staticcheck // testing deprecated field
					ContainerImage: "foo/etcd:v1",
					RootCA:         ca,
					EtcdExtraArgs: meta.Args{
						"election-timeout": meta.NewArgValue("5000", nil),
					},
					EtcdAdvertisedSubnets: []string{"10.0.0.0/8"},
				},
			},
		}

		caConfig := cfg.EtcdCAConfig()
		require.NotNil(t, caConfig)
		assert.Equal(t, ca, caConfig.IssuingCA())
		assert.Equal(t, []*x509.PEMEncodedCertificate{{Crt: []byte("cert")}}, caConfig.AcceptedCAs())

		member := cfg.EtcdMemberConfig()
		require.NotNil(t, member)

		assert.Equal(t, "foo/etcd:v1", member.Image())
		assert.Equal(t, map[string][]string{"election-timeout": {"5000"}}, member.ExtraArgs())
		assert.Equal(t, []string{"10.0.0.0/8"}, member.AdvertisedSubnets())
		assert.Equal(t, []string{"10.0.0.0/8"}, member.ListenSubnets())
	})

	t.Run("deprecated subnet", func(t *testing.T) {
		t.Parallel()

		cfg := &v1alpha1.Config{
			ClusterConfig: &v1alpha1.ClusterConfig{
				EtcdConfig: &v1alpha1.EtcdConfig{ //nolint:staticcheck // testing deprecated field
					RootCA: &x509.PEMEncodedCertificateAndKey{
						Crt: []byte("cert"),
						Key: []byte("key"),
					},
					EtcdSubnet:        "10.0.0.0/8",
					EtcdListenSubnets: []string{"0.0.0.0/0"},
				},
			},
		}

		member := cfg.EtcdMemberConfig()
		require.NotNil(t, member)

		assert.Equal(t, []string{"10.0.0.0/8"}, member.AdvertisedSubnets())
		assert.Equal(t, []string{"0.0.0.0/0"}, member.ListenSubnets())
	})
}
