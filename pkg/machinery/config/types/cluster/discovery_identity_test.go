// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cluster_test

import (
	_ "embed"
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/cluster"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
)

const (
	testClusterID     = "MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDE=" // URLEncoded base64 of 32 random bytes
	testClusterSecret = "vlf2HU1NEZL3Ezi9Tk+RZBLJUbjnsHnTzs3wK9JNk6Q=" // StdEncoded base64 of 32 random bytes
)

//go:embed testdata/discoveryidentityconfig.yaml
var expectedDiscoveryIdentityConfigDocument []byte

func TestDiscoveryIdentityConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := cluster.NewDiscoveryIdentityConfigV1Alpha1(testClusterID, testClusterSecret)

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedDiscoveryIdentityConfigDocument, marshaled)
}

func TestDiscoveryIdentityConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedDiscoveryIdentityConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &cluster.DiscoveryIdentityConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       cluster.DiscoveryIdentityKind,
		},
		MetaClusterID:     testClusterID,
		MetaClusterSecret: testClusterSecret,
	}, docs[0])

	// the document should be surfaced through the aggregated identity accessor
	identity := provider.DiscoveryIdentityConfig()
	require.NotNil(t, identity)
	assert.Equal(t, testClusterID, identity.ClusterID())
	assert.Equal(t, testClusterSecret, identity.ClusterSecret())
}

func TestDiscoveryIdentityConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *cluster.DiscoveryIdentityConfigV1Alpha1

		expectedErrorPrefix string
	}{
		{
			name: "valid",
			cfg: func() *cluster.DiscoveryIdentityConfigV1Alpha1 {
				return cluster.NewDiscoveryIdentityConfigV1Alpha1(testClusterID, testClusterSecret)
			},
		},
		{
			name: "missing clusterID",
			cfg: func() *cluster.DiscoveryIdentityConfigV1Alpha1 {
				return cluster.NewDiscoveryIdentityConfigV1Alpha1("", testClusterSecret)
			},
			expectedErrorPrefix: "clusterID is required",
		},
		{
			name: "base64 clusterID but with StdEncoding",
			cfg: func() *cluster.DiscoveryIdentityConfigV1Alpha1 {
				return cluster.NewDiscoveryIdentityConfigV1Alpha1("MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTIzNDU2Nzg5MDE=", testClusterSecret)
			},
		},
		{
			name: "clusterID length other than 32 bytes",
			cfg: func() *cluster.DiscoveryIdentityConfigV1Alpha1 {
				return cluster.NewDiscoveryIdentityConfigV1Alpha1("AAAAAAAAAAAAAAAAAAAAAA==", testClusterSecret)
			},
		},
		{
			name: "missing clusterSecret",
			cfg: func() *cluster.DiscoveryIdentityConfigV1Alpha1 {
				return cluster.NewDiscoveryIdentityConfigV1Alpha1(testClusterID, "")
			},
			expectedErrorPrefix: "clusterSecret is required",
		},
		{
			name: "invalid base64 clusterSecret",
			cfg: func() *cluster.DiscoveryIdentityConfigV1Alpha1 {
				return cluster.NewDiscoveryIdentityConfigV1Alpha1(testClusterID, "!not-valid-base64!")
			},
			expectedErrorPrefix: "invalid clusterSecret:",
		},
		{
			name: "clusterSecret wrong length",
			cfg: func() *cluster.DiscoveryIdentityConfigV1Alpha1 {
				return cluster.NewDiscoveryIdentityConfigV1Alpha1(testClusterID, "AAAAAAAAAAAAAAAAAAAAAA==")
			},
			expectedErrorPrefix: "invalid clusterSecret:",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			warnings, err := test.cfg().Validate(validationMode{})
			assert.Nil(t, warnings)

			if test.expectedErrorPrefix != "" {
				assert.Error(t, err)
				assert.ErrorContains(t, err, test.expectedErrorPrefix)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDiscoveryIdentityConfigV1Alpha1ConflictValidate(t *testing.T) {
	t.Parallel()

	cfg := cluster.NewDiscoveryIdentityConfigV1Alpha1(testClusterID, testClusterSecret)

	for _, test := range []struct {
		name        string
		v1alpha1Cfg *v1alpha1.Config

		expectedError string
	}{
		{
			name:        "empty",
			v1alpha1Cfg: &v1alpha1.Config{},
		},
		{
			name:        "cluster config without identity",
			v1alpha1Cfg: &v1alpha1.Config{ClusterConfig: &v1alpha1.ClusterConfig{}},
		},
		{
			name: "legacy cluster id present",
			v1alpha1Cfg: &v1alpha1.Config{
				ClusterConfig: &v1alpha1.ClusterConfig{
					ClusterID: testClusterID, //nolint:staticcheck // testing legacy config conflict
				},
			},
			expectedError: "cluster identity is already configured in .cluster.id/.cluster.secret of the v1alpha1 config",
		},
		{
			name: "legacy cluster secret present",
			v1alpha1Cfg: &v1alpha1.Config{
				ClusterConfig: &v1alpha1.ClusterConfig{
					ClusterSecret: testClusterSecret, //nolint:staticcheck // testing legacy config conflict
				},
			},
			expectedError: "cluster identity is already configured in .cluster.id/.cluster.secret of the v1alpha1 config",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := cfg.V1Alpha1ConflictValidate(test.v1alpha1Cfg)
			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDiscoveryIdentityConfigRedact(t *testing.T) {
	t.Parallel()

	const replacement = "**.***"

	for _, test := range []struct {
		name string
		cfg  func() *cluster.DiscoveryIdentityConfigV1Alpha1

		expectedSecret string
	}{
		{
			name: "secret set",
			cfg: func() *cluster.DiscoveryIdentityConfigV1Alpha1 {
				return cluster.NewDiscoveryIdentityConfigV1Alpha1(testClusterID, testClusterSecret)
			},
			expectedSecret: replacement,
		},
		{
			name: "empty secret left untouched",
			cfg: func() *cluster.DiscoveryIdentityConfigV1Alpha1 {
				return cluster.NewDiscoveryIdentityConfigV1Alpha1(testClusterID, "")
			},
			expectedSecret: "",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			cfg := test.cfg()

			cfg.Redact(replacement)

			// the secret is redacted, but the cluster ID (not a secret) is preserved
			assert.Equal(t, testClusterID, cfg.ClusterID())
			assert.Equal(t, test.expectedSecret, cfg.ClusterSecret())
		})
	}
}

func TestValidateBase64WithLen(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name         string
		input        string
		encoding     *base64.Encoding
		wantLenBytes int

		shouldError bool
		errorPrefix string
	}{
		{
			name:         "valid clusterSecret",
			input:        testClusterSecret,
			encoding:     cluster.ClusterSecretEncoding,
			wantLenBytes: 32,
		},
		{
			name:         "invalid base64",
			input:        "!not-valid-base64!",
			encoding:     base64.StdEncoding,
			wantLenBytes: 32,
			shouldError:  true,
			errorPrefix:  "failed to decode from base64:",
		},
		{
			name:         "too short (16 bytes decoded)",
			input:        "AAAAAAAAAAAAAAAAAAAAAA==",
			encoding:     base64.StdEncoding,
			wantLenBytes: 32,
			shouldError:  true,
			errorPrefix:  "expected 32 bytes, got 16:",
		},
		{
			name:         "too long (66 bytes decoded)",
			input:        "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA",
			encoding:     base64.StdEncoding,
			wantLenBytes: 32,
			shouldError:  true,
			errorPrefix:  "expected 32 bytes, got 66:",
		},
		{
			name:         "empty string",
			input:        "",
			encoding:     base64.StdEncoding,
			wantLenBytes: 32,
			shouldError:  true,
			errorPrefix:  "expected 32 bytes, got 0:",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			err := cluster.ValidateBase64WithLen(test.input, test.encoding, test.wantLenBytes)

			if test.shouldError {
				assert.Error(t, err)
				assert.ErrorContains(t, err, test.errorPrefix)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
