// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s_test

import (
	_ "embed"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	k8sadapter "github.com/siderolabs/talos/internal/app/machined/pkg/adapters/k8s"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

func TestManifestSetYAML(t *testing.T) {
	manifest := k8s.NewManifest(k8s.ControlPlaneNamespaceName, "test")
	adapter := k8sadapter.Manifest(manifest)

	require.NoError(t, adapter.SetYAML([]byte(strings.TrimSpace(`
---
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
- level: Metadata
---
`))))

	assert.Len(t, adapter.Objects(), 1)
	assert.Equal(t, adapter.Objects()[0].GetKind(), "Policy")
}

func TestManifestSetYAMLEmptyComments(t *testing.T) {
	manifest := k8s.NewManifest(k8s.ControlPlaneNamespaceName, "test")
	adapter := k8sadapter.Manifest(manifest)

	require.NoError(t, adapter.SetYAML([]byte(strings.TrimSpace(`
---
apiVersion: audit.k8s.io/v1
kind: Policy
rules:
- level: Metadata
---
# Left empty
---
`))))

	assert.Len(t, adapter.Objects(), 1)
	assert.Equal(t, adapter.Objects()[0].GetKind(), "Policy")
}

//go:embed testdata/list.yaml
var listManifest []byte

func TestManifestSetYAMLList(t *testing.T) {
	manifest := k8s.NewManifest(k8s.ControlPlaneNamespaceName, "test")
	adapter := k8sadapter.Manifest(manifest)

	require.NoError(t, adapter.SetYAML(listManifest))

	assert.Len(t, adapter.Objects(), 2)
	assert.Equal(t, "ClusterRoleBinding", adapter.Objects()[0].GetKind())
	assert.Equal(t, "system:cloud-node-controller", adapter.Objects()[0].GetName())
	assert.Equal(t, "ClusterRoleBinding", adapter.Objects()[1].GetKind())
	assert.Equal(t, "system:cloud-controller-manager", adapter.Objects()[1].GetName())
}
