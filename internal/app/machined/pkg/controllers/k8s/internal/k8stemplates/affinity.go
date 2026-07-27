// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8stemplates

import (
	corev1 "k8s.io/api/core/v1"

	"github.com/siderolabs/talos/pkg/machinery/constants"
)

var nodeAffinity = &corev1.NodeAffinity{
	RequiredDuringSchedulingIgnoredDuringExecution: &corev1.NodeSelector{
		NodeSelectorTerms: []corev1.NodeSelectorTerm{
			{
				MatchExpressions: []corev1.NodeSelectorRequirement{
					{
						Key:      "kubernetes.io/os",
						Operator: corev1.NodeSelectorOpIn,
						Values:   []string{"linux"},
					},
					{
						Key:      "kubernetes.io/arch",
						Operator: corev1.NodeSelectorOpIn,
						Values:   constants.SupportedArchitectures,
					},
				},
			},
		},
	},
}
