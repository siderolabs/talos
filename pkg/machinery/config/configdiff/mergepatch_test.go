// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package configdiff_test

import (
	_ "embed"
	"testing"
	"time"

	"github.com/siderolabs/gen/xslices"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/talos/pkg/machinery/config"
	coreconfig "github.com/siderolabs/talos/pkg/machinery/config/config"
	"github.com/siderolabs/talos/pkg/machinery/config/configdiff"
	"github.com/siderolabs/talos/pkg/machinery/config/configloader"
	"github.com/siderolabs/talos/pkg/machinery/config/configpatcher"
	"github.com/siderolabs/talos/pkg/machinery/config/container"
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
	"github.com/siderolabs/talos/pkg/machinery/config/generate"
	"github.com/siderolabs/talos/pkg/machinery/config/generate/secrets"
	"github.com/siderolabs/talos/pkg/machinery/config/internal/documentid"
	"github.com/siderolabs/talos/pkg/machinery/config/machine"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

var (
	//go:embed testdata/original.yaml
	originalYAML []byte

	//go:embed testdata/modified.yaml
	modifiedYAML []byte
)

func TestMergePatch(t *testing.T) {
	original, err := configloader.NewFromBytes(originalYAML)
	require.NoError(t, err)

	modified, err := configloader.NewFromBytes(modifiedYAML)
	require.NoError(t, err)

	patches, err := configdiff.Patch(original, modified)
	require.NoError(t, err)

	apply, err := configpatcher.Apply(configpatcher.WithConfig(original), patches)
	require.NoError(t, err)

	appliedBytes, err := apply.Bytes()
	require.NoError(t, err)

	modifiedBytes, err := modified.Bytes()
	require.NoError(t, err)

	require.Equal(t, string(modifiedBytes), string(appliedBytes))
}

var inlineOriginal = []byte(`version: v1alpha1
machine:
    type: controlplane
    token: 2to1o4.gtwik66aods4cznj
    certSANs:
        - example.com
    kubelet:
        extraArgs:
            cloud-provider: external
cluster:
    controlPlane:
        endpoint: https://[fdae:41e4:649b:9303::1]:10000
---
apiVersion: v1alpha1
kind: NetworkDefaultActionConfig
ingress: block
`)

func TestMergePatchInline(t *testing.T) {
	tests := []struct {
		name            string
		originalAsBytes []byte
		modifiedAsBytes []byte
		patchesAsBytes  [][]byte
	}{
		{
			name:            "test add field",
			originalAsBytes: inlineOriginal,
			modifiedAsBytes: []byte(`version: v1alpha1
machine:
    type: controlplane
    token: 2to1o4.gtwik66aods4cznj
    certSANs:
        - example.com
    kubelet:
        extraArgs:
            cloud-config: /etc/kubernetes/cloud-config
            cloud-provider: external
cluster:
    controlPlane:
        endpoint: https://[fdae:41e4:649b:9303::1]:10000
---
apiVersion: v1alpha1
kind: NetworkDefaultActionConfig
ingress: block
`),
			patchesAsBytes: [][]byte{
				[]byte(`machine:
  kubelet:
    extraArgs:
      cloud-config: /etc/kubernetes/cloud-config
version: v1alpha1
`),
			},
		},
		{
			name:            "test replace field",
			originalAsBytes: inlineOriginal,
			modifiedAsBytes: []byte(`version: v1alpha1
machine:
    type: controlplane
    token: 2to1o4.gtwik66aods4cznj
    certSANs:
        - example.com
    kubelet:
        extraArgs:
            cloud-provider: external
cluster:
    controlPlane:
        endpoint: https://[fdae:41e4:649b:9303::1]:10001
---
apiVersion: v1alpha1
kind: NetworkDefaultActionConfig
ingress: block
`),
			patchesAsBytes: [][]byte{
				[]byte(`cluster:
  controlPlane:
    endpoint: https://[fdae:41e4:649b:9303::1]:10001
version: v1alpha1
`),
			},
		},
		{
			name:            "test add nested field",
			originalAsBytes: inlineOriginal,
			modifiedAsBytes: []byte(`version: v1alpha1
machine:
    type: controlplane
    token: 2to1o4.gtwik66aods4cznj
    certSANs:
        - example.com
    kubelet:
        extraArgs:
            cloud-provider: external
cluster:
    controlPlane:
        endpoint: https://[fdae:41e4:649b:9303::1]:10000
    discovery:
        registries:
            kubernetes:
                disabled: false
            service: {}
---
apiVersion: v1alpha1
kind: NetworkDefaultActionConfig
ingress: block
`),
			patchesAsBytes: [][]byte{
				[]byte(`cluster:
  discovery:
    registries:
      kubernetes:
        disabled: false
      service: {}
version: v1alpha1
`),
			},
		},
		{
			name:            "test replace item in list",
			originalAsBytes: inlineOriginal,
			modifiedAsBytes: []byte(`version: v1alpha1
machine:
    type: controlplane
    token: 2to1o4.gtwik66aods4cznj
    certSANs:
        - new-example.com
    kubelet:
        extraArgs:
            cloud-provider: external
cluster:
    controlPlane:
        endpoint: https://[fdae:41e4:649b:9303::1]:10000
---
apiVersion: v1alpha1
kind: NetworkDefaultActionConfig
ingress: block
`),
			patchesAsBytes: [][]byte{
				[]byte(`machine:
  certSANs:
    $patch: delete
version: v1alpha1
`),

				[]byte(`machine:
  certSANs:
    - new-example.com
version: v1alpha1
`),
			},
		},
		{
			name:            "test remove key",
			originalAsBytes: inlineOriginal,
			modifiedAsBytes: []byte(`version: v1alpha1
machine:
    type: controlplane
    token: 2to1o4.gtwik66aods4cznj
    certSANs:
        - example.com
cluster:
    controlPlane:
        endpoint: https://[fdae:41e4:649b:9303::1]:10000
---
apiVersion: v1alpha1
kind: NetworkDefaultActionConfig
ingress: block
`),
			patchesAsBytes: [][]byte{
				[]byte(`machine:
  kubelet:
    $patch: delete
version: v1alpha1
`),
			},
		},
		{
			name:            "test add document",
			originalAsBytes: inlineOriginal,
			modifiedAsBytes: []byte(`version: v1alpha1
machine:
    type: controlplane
    token: 2to1o4.gtwik66aods4cznj
    certSANs:
        - example.com
    kubelet:
        extraArgs:
            cloud-provider: external
cluster:
    controlPlane:
        endpoint: https://[fdae:41e4:649b:9303::1]:10000
---
apiVersion: v1alpha1
kind: NetworkDefaultActionConfig
ingress: block
---
apiVersion: v1alpha1
kind: KmsgLogConfig
name: apiSink
url: tcp://[fdae:41e4:649b:9303::1]:4001/
`),
			patchesAsBytes: [][]byte{
				[]byte(`apiVersion: v1alpha1
kind: KmsgLogConfig
name: apiSink
url: tcp://[fdae:41e4:649b:9303::1]:4001/
`),
			},
		},
		{
			name:            "test remove document",
			originalAsBytes: inlineOriginal,
			modifiedAsBytes: []byte(`version: v1alpha1
machine:
    type: controlplane
    token: 2to1o4.gtwik66aods4cznj
    certSANs:
        - example.com
    kubelet:
        extraArgs:
            cloud-provider: external
cluster:
    controlPlane:
        endpoint: https://[fdae:41e4:649b:9303::1]:10000
`),
			patchesAsBytes: [][]byte{
				[]byte(`$patch: delete
apiVersion: v1alpha1
kind: NetworkDefaultActionConfig
`),
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			original, err := configloader.NewFromBytes(tt.originalAsBytes)
			require.NoError(t, err)

			modified, err := configloader.NewFromBytes(tt.modifiedAsBytes)
			require.NoError(t, err)

			patches, err := configdiff.Patch(original, modified)
			require.NoError(t, err)

			for i, patch := range patches {
				provider := patch.(configpatcher.StrategicMergePatch).Provider()

				patchBytes, err := provider.Bytes()
				require.NoError(t, err)

				require.Equal(t, string(patchBytes), string(tt.patchesAsBytes[i]))
			}

			apply, err := configpatcher.Apply(configpatcher.WithConfig(original), patches)
			require.NoError(t, err)

			appliedBytes, err := apply.Bytes()
			require.NoError(t, err)

			modifiedBytes, err := modified.Bytes()
			require.NoError(t, err)

			require.Equal(t, string(modifiedBytes), string(appliedBytes))
		})
	}
}

var dynamicPatches = [][]byte{
	[]byte(`machine:
    network:
        interfaces:
            - interface: enp0s2
              dhcp: true
`),
	[]byte(`apiVersion: v1alpha1
kind: KmsgLogConfig
name: apiSink
url: tcp://[fdae:41e4:649b:9303::1]:4001/
`),
	[]byte(`cluster:
    apiServer:
        certSANs:
            - example.com
        admissionControl:
            - name: PodSecurity
              configuration:
                exemptions:
                    namespaces:
                        - kube-public
                        - kube-node-lease
`),
	[]byte(`cluster:
    apiServer:
        admissionControl:
            - name: PrivilegedPodSecurity
              configuration:
                apiVersion: pod-security.admission.config.k8s.io/v1alpha1
                defaults:
                    audit: privileged
                    audit-version: latest
                    enforce: privileged
                    enforce-version: latest
                    warn: privileged
                    warn-version: latest
                exemptions:
                    namespaces: []
                    runtimeClasses: []
                    usernames: []
                kind: PodSecurityConfiguration
`),
	[]byte(`apiVersion: v1alpha1
kind: ExtensionServiceConfig
name: foo
configFiles:
    - content: hello-foo
      mountPath: /etc/foo
environment:
    - FOO=BAR
    - BAR=FOO
---
apiVersion: v1alpha1
kind: ExtensionServiceConfig
name: var
configFiles:
    - content: hello-var
      mountPath: /etc/var
    - content: hello-foo
      mountPath: /etc/var/foo
environment:
    - FOO=BAR
`),
	[]byte(`apiVersion: v1alpha1
kind: SideroLinkConfig
apiUrl: grpc://omni.example.com:8090?jointoken=testtoken
---
apiVersion: v1alpha1
kind: EventSinkConfig
endpoint: '[fdae:41e4:649b:9303::1]:8091'
---
apiVersion: v1alpha1
kind: KmsgLogConfig
name: omni-kmsg
url: tcp://[fdae:41e4:649b:9303::1]:8092
`),
	[]byte(`apiVersion: v1alpha1
kind: KmsgLogConfig
name: apiSink
$patch: delete
`),
	[]byte(`apiVersion: v1alpha1
kind: EthernetConfig
name: enp0s2
features:
    tx-tcp-segmentation: false
`),
	[]byte(`apiVersion: v1alpha1
kind: ExtensionServiceConfig
name: var
configFiles:
    - content: hello-var
      mountPath: /etc/var
      $patch: delete
environment:
    - FOO=BARFOO
`),
}

func TestMergePatchDynamic(t *testing.T) {
	bundle, err := secrets.NewBundle(secrets.NewFixedClock(time.Now()), config.TalosVersionCurrent)
	require.NoError(t, err)

	input, err := generate.NewInput("test", "https://localhost:6443", constants.DefaultKubernetesVersion, generate.WithSecretsBundle(bundle))
	require.NoError(t, err)

	original, err := input.Config(machine.TypeControlPlane)
	require.NoError(t, err)

	var modified config.Provider

	modified = original.Clone()
	// Apply patches one by one to simulate real world usage
	for _, patchBytes := range dynamicPatches {
		patches, patchErr := configpatcher.LoadPatch(patchBytes)
		require.NoError(t, patchErr)

		patched, patchErr := configpatcher.Apply(configpatcher.WithConfig(modified), []configpatcher.Patch{patches})
		require.NoError(t, patchErr)

		modified, patchErr = patched.Config()
		require.NoError(t, patchErr)
	}

	// Get merge patches between original and modified
	patches, err := configdiff.Patch(original, modified)
	require.NoError(t, err)

	// Apply the merge patches to the original config
	patched, err := configpatcher.Apply(configpatcher.WithConfig(original), patches)
	require.NoError(t, err)

	patchedConfig, err := patched.Config()
	require.NoError(t, err)

	// configpatcher.Apply may change the order of documents, so we need to compare them one by one
	modifiedDocuments := modified.Documents()
	patchedDocuments := patchedConfig.Documents()

	require.Equal(t, len(modifiedDocuments), len(patchedDocuments))

	modifiedDocumentsMap := xslices.ToMap(modifiedDocuments, func(doc coreconfig.Document) (documentid.DocumentID, coreconfig.Document) {
		return documentid.Extract(doc), doc
	})

	patchedDocumentsMap := xslices.ToMap(patchedDocuments, func(doc coreconfig.Document) (documentid.DocumentID, coreconfig.Document) {
		return documentid.Extract(doc), doc
	})

	for id, modifiedDoc := range modifiedDocumentsMap {
		patchedDoc, ok := patchedDocumentsMap[id]
		require.True(t, ok, "document %v not found in patched config", id)

		modifiedDocContainer, docErr := container.New(modifiedDoc)
		require.NoError(t, docErr)

		patchedDocContainer, docErr := container.New(patchedDoc)
		require.NoError(t, docErr)

		modifiedBytes, docErr := modifiedDocContainer.EncodeBytes(encoder.WithComments(encoder.CommentsDisabled))
		require.NoError(t, docErr)

		patchedBytes, docErr := patchedDocContainer.EncodeBytes(encoder.WithComments(encoder.CommentsDisabled))
		require.NoError(t, docErr)

		require.Equal(t, string(modifiedBytes), string(patchedBytes), "document %v does not match", id)
	}
}
