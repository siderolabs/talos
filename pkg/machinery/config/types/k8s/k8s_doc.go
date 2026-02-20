// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8s

import (
	"github.com/siderolabs/talos/pkg/machinery/config/encoder"
)

// Doc returns the documentation for EncryptionConfigurationDoc.
//
//nolint:lll
func (EncryptionConfigurationDoc) Doc() *encoder.Doc {
	doc := &encoder.Doc{
		Type: "EncryptionConfiguration",
		Comments: [3]string{
			"",
			"EncryptionConfiguration is a native Kubernetes API server encryption configuration document.",
			"",
		},
		Description: "EncryptionConfiguration allows providing a custom Kubernetes API server encryption configuration.\n" +
			"When specified, the custom encryption config is applied as-is, bypassing the Talos-generated default.\n" +
			"The document uses the upstream Kubernetes apiVersion and kind:\n" +
			"apiVersion: apiserver.config.k8s.io/v1, kind: EncryptionConfiguration.\n" +
			"See https://kubernetes.io/docs/tasks/administer-cluster/encrypt-data/ for details.\n",
	}

	doc.AddExample("", exampleEncryptionConfiguration())

	return doc
}

func exampleEncryptionConfiguration() *EncryptionConfigurationDoc {
	return &EncryptionConfigurationDoc{
		Fields: map[string]any{
			"apiVersion": EncryptionConfigurationAPIVersion,
			"kind":       EncryptionConfigurationKind,
			"resources": []any{
				map[string]any{
					"resources": []any{"secrets"},
					"providers": []any{
						map[string]any{
							"secretbox": map[string]any{
								"keys": []any{
									map[string]any{
										"name":   "key1",
										"secret": "dGl0aXRvdG90aXRpdG90b3RpdGl0b3RvdGl0aXRvdG8K",
									},
								},
							},
						},
						map[string]any{
							"identity": map[string]any{},
						},
					},
				},
			},
		},
	}
}

// GetFileDoc returns documentation for the file k8s_doc.go.
func GetFileDoc() *encoder.FileDoc {
	return &encoder.FileDoc{
		Name:        "k8s",
		Description: "Package k8s provides k8s-related machine configuration documents.\n",
		Structs: []*encoder.Doc{
			EncryptionConfigurationDoc{}.Doc(),
		},
	}
}
