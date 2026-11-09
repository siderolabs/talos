// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package etcd_test

import (
	_ "embed"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/etcd"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

func newTestMemberConfig() *etcd.EtcdMemberConfigV1Alpha1 {
	cfg := etcd.NewEtcdMemberConfigV1Alpha1()
	cfg.MemberImage = "registry.k8s.io/etcd:v3.6.0"
	cfg.MemberExtraArgs = meta.Args{
		"election-timeout":  meta.NewArgValue("5000", nil),
		"experimental-flag": meta.NewArgValue("", []string{"a", "b"}),
	}
	cfg.MemberAdvertisedSubnets = []string{"10.0.0.0/8", "!10.0.0.3/32"}
	cfg.MemberListenSubnets = []string{"10.0.0.0/8"}

	return cfg
}

//go:embed testdata/etcdmemberconfig.yaml
var expectedEtcdMemberConfigDocument []byte

func TestEtcdMemberConfigMarshalStability(t *testing.T) {
	t.Parallel()

	marshaled, err := encoder.NewEncoder(newTestMemberConfig(), encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedEtcdMemberConfigDocument, marshaled)
}

func TestEtcdMemberConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedEtcdMemberConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, newTestMemberConfig(), docs[0])
}

func TestEtcdMemberConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *etcd.EtcdMemberConfigV1Alpha1

		expectedError string
	}{
		{
			name: "empty",
			cfg:  etcd.NewEtcdMemberConfigV1Alpha1,
		},
		{
			name: "valid",
			cfg:  newTestMemberConfig,
		},
		{
			name: "invalid subnets",
			cfg: func() *etcd.EtcdMemberConfigV1Alpha1 {
				cfg := etcd.NewEtcdMemberConfigV1Alpha1()
				cfg.MemberAdvertisedSubnets = []string{"10.0.0.0/8", "foo"}
				cfg.MemberListenSubnets = []string{"!bar"}

				return cfg
			},

			expectedError: "etcd advertised subnet is not valid: \"foo\"\netcd listen subnet is not valid: \"!bar\"",
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			warnings, err := test.cfg().Validate(validationMode{})

			assert.Empty(t, warnings)

			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestEtcdMemberConfigAccessors(t *testing.T) {
	t.Parallel()

	cfg := etcd.NewEtcdMemberConfigV1Alpha1()

	// defaults
	assert.Equal(t, constants.EtcdImage+":"+constants.DefaultEtcdVersion, cfg.Image())
	assert.Empty(t, cfg.ExtraArgs())
	assert.Empty(t, cfg.AdvertisedSubnets())
	assert.Empty(t, cfg.ListenSubnets())

	// listen subnets default to advertised subnets
	cfg.MemberAdvertisedSubnets = []string{"10.0.0.0/8"}
	assert.Equal(t, []string{"10.0.0.0/8"}, cfg.AdvertisedSubnets())
	assert.Equal(t, []string{"10.0.0.0/8"}, cfg.ListenSubnets())

	cfg = newTestMemberConfig()
	assert.Equal(t, "registry.k8s.io/etcd:v3.6.0", cfg.Image())
	assert.Equal(t, map[string][]string{
		"election-timeout":  {"5000"},
		"experimental-flag": {"a", "b"},
	}, cfg.ExtraArgs())
	assert.Equal(t, []string{"10.0.0.0/8", "!10.0.0.3/32"}, cfg.AdvertisedSubnets())
	assert.Equal(t, []string{"10.0.0.0/8"}, cfg.ListenSubnets())
}
