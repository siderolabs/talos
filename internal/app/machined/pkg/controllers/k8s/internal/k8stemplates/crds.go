// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8stemplates

import (
	apiextensionsv1 "k8s.io/apiextensions-api/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// TalosServiceAccountCRDTemplate returns the template of the CRD which
// allows injecting Talos API credentials for Kubernetes pods.
func TalosServiceAccountCRDTemplate() runtime.Object {
	return &apiextensionsv1.CustomResourceDefinition{
		APIVersion: apiextensionsv1.SchemeGroupVersion.String(),
		Kind:       "CustomResourceDefinition",
		Name:       constants.ServiceAccountResourcePlural + "." + constants.ServiceAccountResourceGroup,
		Spec: apiextensionsv1.CustomResourceDefinitionSpec{
			Conversion: &apiextensionsv1.CustomResourceConversion{
				Strategy: apiextensionsv1.NoneConverter,
			},
			Group: constants.ServiceAccountResourceGroup,
			Names: apiextensionsv1.CustomResourceDefinitionNames{
				Kind:       constants.ServiceAccountResourceKind,
				ListKind:   constants.ServiceAccountResourceKind + "List",
				Plural:     constants.ServiceAccountResourcePlural,
				Singular:   constants.ServiceAccountResourceSingular,
				ShortNames: []string{constants.ServiceAccountResourceShortName},
			},
			Scope: apiextensionsv1.NamespaceScoped,
			Versions: []apiextensionsv1.CustomResourceDefinitionVersion{
				{
					Name:    constants.ServiceAccountResourceVersion,
					Served:  true,
					Storage: true,
					Schema: &apiextensionsv1.CustomResourceValidation{
						OpenAPIV3Schema: &apiextensionsv1.JSONSchemaProps{
							Type: "object",
							Properties: map[string]apiextensionsv1.JSONSchemaProps{
								"spec": {
									Type: "object",
									Properties: map[string]apiextensionsv1.JSONSchemaProps{
										"roles": {
											Type: "array",
											Items: &apiextensionsv1.JSONSchemaPropsOrArray{
												Schema: &apiextensionsv1.JSONSchemaProps{
													Type: "string",
												},
											},
										},
									},
								},
								"status": {
									Type: "object",
									Properties: map[string]apiextensionsv1.JSONSchemaProps{
										"failureReason": {
											Type: "string",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
