// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package network_test

import (
	_ "embed"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/types/meta"
	"github.com/siderolabs/talos/pkg/machinery/config/types/network"
)

//go:embed testdata/probeconfig.yaml
var expectedProbeConfigDocument []byte

func TestProbeConfigMarshalStability(t *testing.T) {
	t.Parallel()

	cfg := &network.ProbeConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       network.ProbeKind,
			MetaAPIVersion: "v1alpha1",
		},
		MetaName:         "proxy-check",
		ProbeInterval:    time.Second,
		FailureThreshold: 3,
		TCP: &network.TCPProbeConfigV1Alpha1{
			Endpoint: "proxy.example.com:3128",
			Timeout:  10 * time.Second,
		},
	}

	marshaled, err := encoder.NewEncoder(cfg, encoder.WithComments(encoder.CommentsDisabled)).Encode()
	require.NoError(t, err)

	t.Log(string(marshaled))

	assert.Equal(t, expectedProbeConfigDocument, marshaled)
}

func TestProbeConfigUnmarshal(t *testing.T) {
	t.Parallel()

	provider, err := configloader.NewFromBytes(expectedProbeConfigDocument)
	require.NoError(t, err)

	docs := provider.Documents()
	require.Len(t, docs, 1)

	assert.Equal(t, &network.ProbeConfigV1Alpha1{
		Meta: meta.Meta{
			MetaAPIVersion: "v1alpha1",
			MetaKind:       network.ProbeKind,
		},
		MetaName:         "proxy-check",
		ProbeInterval:    time.Second,
		FailureThreshold: 3,
		TCP: &network.TCPProbeConfigV1Alpha1{
			Endpoint: "proxy.example.com:3128",
			Timeout:  10 * time.Second,
		},
	}, docs[0])
}

func TestProbeConfigValidate(t *testing.T) {
	t.Parallel()

	for _, test := range []struct {
		name string
		cfg  func() *network.ProbeConfigV1Alpha1

		expectedError    string
		expectedWarnings []string
	}{
		{
			name: "valid config",
			cfg: func() *network.ProbeConfigV1Alpha1 {
				return &network.ProbeConfigV1Alpha1{
					Meta: meta.Meta{
						MetaKind:       network.ProbeKind,
						MetaAPIVersion: "v1alpha1",
					},
					MetaName:         "test-probe",
					ProbeInterval:    time.Second,
					FailureThreshold: 3,
					TCP: &network.TCPProbeConfigV1Alpha1{
						Endpoint: "example.com:80",
						Timeout:  5 * time.Second,
					},
				}
			},
		},
		{
			name: "missing name",
			cfg: func() *network.ProbeConfigV1Alpha1 {
				return &network.ProbeConfigV1Alpha1{
					Meta: meta.Meta{
						MetaKind:       network.ProbeKind,
						MetaAPIVersion: "v1alpha1",
					},
					TCP: &network.TCPProbeConfigV1Alpha1{
						Endpoint: "example.com:80",
					},
				}
			},
			expectedError: "probe name is required",
		},
		{
			name: "missing probe type",
			cfg: func() *network.ProbeConfigV1Alpha1 {
				return &network.ProbeConfigV1Alpha1{
					Meta: meta.Meta{
						MetaKind:       network.ProbeKind,
						MetaAPIVersion: "v1alpha1",
					},
					MetaName: "test-probe",
				}
			},
			expectedError: "probe type must be specified (currently only TCP is supported)",
		},
		{
			name: "missing TCP endpoint",
			cfg: func() *network.ProbeConfigV1Alpha1 {
				return &network.ProbeConfigV1Alpha1{
					Meta: meta.Meta{
						MetaKind:       network.ProbeKind,
						MetaAPIVersion: "v1alpha1",
					},
					MetaName: "test-probe",
					TCP:      &network.TCPProbeConfigV1Alpha1{},
				}
			},
			expectedError: "TCP probe endpoint is required",
		},
		{
			name: "defaults applied",
			cfg: func() *network.ProbeConfigV1Alpha1 {
				cfg := &network.ProbeConfigV1Alpha1{
					Meta: meta.Meta{
						MetaKind:       network.ProbeKind,
						MetaAPIVersion: "v1alpha1",
					},
					MetaName: "test-probe",
					TCP: &network.TCPProbeConfigV1Alpha1{
						Endpoint: "example.com:80",
					},
				}

				// Validate should set defaults
				_, err := cfg.Validate(validationMode{})
				require.NoError(t, err)

				// Check defaults were set
				assert.Equal(t, time.Second, cfg.ProbeInterval)
				assert.Equal(t, 10*time.Second, cfg.TCP.Timeout)

				return cfg
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			warnings, err := test.cfg().Validate(validationMode{})

			assert.Equal(t, test.expectedWarnings, warnings)

			if test.expectedError != "" {
				assert.EqualError(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestProbeConfigName(t *testing.T) {
	t.Parallel()

	cfg := &network.ProbeConfigV1Alpha1{
		MetaName: "my-probe",
	}

	assert.Equal(t, "my-probe", cfg.Name())
}

func TestProbeConfigClone(t *testing.T) {
	t.Parallel()

	original := &network.ProbeConfigV1Alpha1{
		Meta: meta.Meta{
			MetaKind:       network.ProbeKind,
			MetaAPIVersion: "v1alpha1",
		},
		MetaName:         "test-probe",
		ProbeInterval:    time.Second,
		FailureThreshold: 3,
		TCP: &network.TCPProbeConfigV1Alpha1{
			Endpoint: "example.com:80",
			Timeout:  5 * time.Second,
		},
	}

	cloned := original.Clone().(*network.ProbeConfigV1Alpha1)

	assert.Equal(t, original, cloned)
	assert.NotSame(t, original, cloned)
	assert.NotSame(t, original.TCP, cloned.TCP)

	// Modify clone and verify original is unchanged
	cloned.MetaName = "modified"
	cloned.TCP.Endpoint = "modified.com:8080"

	assert.Equal(t, "test-probe", original.MetaName)
	assert.Equal(t, "example.com:80", original.TCP.Endpoint)
}
