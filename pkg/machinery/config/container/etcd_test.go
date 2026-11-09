// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package container_test

import (
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	coreconfig "github.com/siderolabs/talos/pkg/machinery/config"
	"github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/config/types/etcd"
	"github.com/siderolabs/talos/pkg/machinery/config/types/v1alpha1"
)

//nolint:gocyclo
func TestValidateEtcdConfig(t *testing.T) {
	t.Parallel()

	input, err := generate.NewInput("test", "https://localhost:6443", "1.35.0", generate.WithVersionContract(coreconfig.TalosVersion1_15))
	require.NoError(t, err)

	generateDocs := func(machineType machine.Type) []config.Document {
		cfg, err := input.Config(machineType)
		require.NoError(t, err)

		return cfg.Clone().Documents()
	}

	withoutKind := func(docs []config.Document, kind string) []config.Document {
		return slices.DeleteFunc(docs, func(doc config.Document) bool {
			return doc.Kind() == kind
		})
	}

	findKind := func(docs []config.Document, kind string) config.Document {
		idx := slices.IndexFunc(docs, func(doc config.Document) bool {
			return doc.Kind() == kind
		})
		require.GreaterOrEqual(t, idx, 0)

		return docs[idx]
	}

	for _, test := range []struct {
		name      string
		documents func() []config.Document

		expectedErrors []string
		expectEtcd     bool
		expectEtcdCA   bool
	}{
		{
			name: "controlplane",
			documents: func() []config.Document {
				return generateDocs(machine.TypeControlPlane)
			},

			expectEtcd:   true,
			expectEtcdCA: true,
		},
		{
			name: "controlplane CA only",
			documents: func() []config.Document {
				return withoutKind(generateDocs(machine.TypeControlPlane), etcd.EtcdMemberConfig)
			},

			expectEtcdCA: true,
		},
		{
			name: "controlplane member without CA",
			documents: func() []config.Document {
				return withoutKind(generateDocs(machine.TypeControlPlane), etcd.EtcdCAConfig)
			},

			expectedErrors: []string{"EtcdMemberConfig requires the EtcdCAConfig document to be present"},
			expectEtcd:     true,
		},
		{
			name: "controlplane mixed with legacy",
			documents: func() []config.Document {
				docs := generateDocs(machine.TypeControlPlane)

				v1alpha1Cfg := findKind(docs, v1alpha1.Version).(*v1alpha1.Config) //nolint:forcetypeassert
				v1alpha1Cfg.ClusterConfig.EtcdConfig = &v1alpha1.EtcdConfig{       //nolint:staticcheck // testing deprecated field
					RootCA: findKind(docs, etcd.EtcdCAConfig).(*etcd.EtcdCAConfigV1Alpha1).IssuingCA(), //nolint:forcetypeassert
				}

				return docs
			},

			expectedErrors: []string{"etcd config is already set in v1alpha1 config (.cluster.etcd)"},
			expectEtcd:     true,
			expectEtcdCA:   true,
		},
		{
			name: "worker with etcd",
			documents: func() []config.Document {
				cpDocs := generateDocs(machine.TypeControlPlane)

				return append(
					generateDocs(machine.TypeWorker),
					findKind(cpDocs, etcd.EtcdCAConfig),
					findKind(cpDocs, etcd.EtcdMemberConfig),
				)
			},

			expectedErrors: []string{"the following document kinds are only allowed on control plane machines: [EtcdCAConfig EtcdMemberConfig]"},
			expectEtcd:     true,
			expectEtcdCA:   true,
		},
		{
			name: "worker",
			documents: func() []config.Document {
				return generateDocs(machine.TypeWorker)
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()

			ctr, err := container.New(test.documents()...)
			require.NoError(t, err)

			assert.Equal(t, test.expectEtcd, ctr.EtcdMemberConfig() != nil)
			assert.Equal(t, test.expectEtcdCA, ctr.EtcdCAConfig() != nil)

			_, err = ctr.ValidateAsClient(validationMode{})

			if len(test.expectedErrors) == 0 {
				require.NoError(t, err)

				return
			}

			require.Error(t, err)

			for _, expectedError := range test.expectedErrors {
				assert.ErrorContains(t, err, expectedError)
			}
		})
	}
}
