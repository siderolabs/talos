// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8stemplates

import (
	apiextensions "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// TalosServiceAccountCRDTemplate returns the template of the CRD which
// allows injecting Talos API credentials for Kubernetes pods.
func TalosServiceAccountCRDTemplate() runtime.Object {
	return &apiextensions.CustomResourceDefinition{
		TypeMeta: v1.TypeMeta{
			APIVersion: apiextensions.SchemeGroupVersion.String(),
			Kind:       "CustomResourceDefinition",
		},
		ObjectMeta: v1.ObjectMeta{
			Name: constants.ServiceAccountResourcePlural + "." + constants.ServiceAccountResourceGroup,
		},
		Spec: apiextensions.CustomResourceDefinitionSpec{
			Conversion: &apiextensions.CustomResourceConversion{
				Strategy: apiextensions.NoneConverter,
			},
			Group: constants.ServiceAccountResourceGroup,
			Names: apiextensions.CustomResourceDefinitionNames{
				Kind:       constants.ServiceAccountResourceKind,
				ListKind:   constants.ServiceAccountResourceKind + "List",
				Plural:     constants.ServiceAccountResourcePlural,
				Singular:   constants.ServiceAccountResourceSingular,
				ShortNames: []string{constants.ServiceAccountResourceShortName},
			},
			Scope: apiextensions.NamespaceScoped,
			Versions: []apiextensions.CustomResourceDefinitionVersion{
				{
					Name:    constants.ServiceAccountResourceVersion,
					Served:  true,
					Storage: true,
					Schema: &apiextensions.CustomResourceValidation{
						OpenAPIV3Schema: &apiextensions.JSONSchemaProps{
							Type: "object",
							Properties: map[string]apiextensions.JSONSchemaProps{
								"spec": {
									Type: "object",
									Properties: map[string]apiextensions.JSONSchemaProps{
										"roles": {
											Type: "array",
											Items: &apiextensions.JSONSchemaPropsOrArray{
												Schema: &apiextensions.JSONSchemaProps{
													Type: "string",
												},
											},
										},
									},
								},
								"status": {
									Type: "object",
									Properties: map[string]apiextensions.JSONSchemaProps{
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
