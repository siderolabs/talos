// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8stemplates

import (
	rbacv1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// TalosNodesRBACClusterRoleBinding is the template of the RBAC rules which allow
// Talos to discover the nodes in the Kubernetes cluster and assign
// endpoints for the internal discovery.
func TalosNodesRBACClusterRoleBinding() runtime.Object {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: v1.TypeMeta{
			Kind:       "ClusterRoleBinding",
			APIVersion: rbacv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: v1.ObjectMeta{
			Name: "system:talos-nodes",
			Labels: map[string]string{
				"kubernetes.io/bootstrapping": "rbac-defaults",
			},
			Annotations: map[string]string{
				"rbac.authorization.kubernetes.io/autoupdate": "true",
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "system:talos-nodes",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:     "Group",
				Name:     "system:nodes",
				APIGroup: rbacv1.GroupName,
			},
		},
	}
}

// TalosNodesRBACClusterRole is the template of the RBAC rules which allow
// Talos to discover the nodes in the Kubernetes cluster and assign
// endpoints for the internal discovery.
func TalosNodesRBACClusterRole() runtime.Object {
	return &rbacv1.ClusterRole{
		TypeMeta: v1.TypeMeta{
			Kind:       "ClusterRole",
			APIVersion: rbacv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: v1.ObjectMeta{
			Name: "system:talos-nodes",
			Labels: map[string]string{
				"kubernetes.io/bootstrapping": "rbac-defaults",
			},
		},
		Rules: []rbacv1.PolicyRule{
			{
				APIGroups: []string{"discovery.k8s.io"},
				Resources: []string{"endpointslices"},
				Verbs:     []string{"get", "list", "watch"},
			},
		},
	}
}
