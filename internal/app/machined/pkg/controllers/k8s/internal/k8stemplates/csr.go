// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package k8stemplates

import (
	rbacv1 "k8s.io/api/rbac/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

// CSRNodeBootstrapTemplate returns the CSR node bootstrap template.
func CSRNodeBootstrapTemplate() runtime.Object {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: v1.TypeMeta{
			Kind:       "ClusterRoleBinding",
			APIVersion: rbacv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: v1.ObjectMeta{
			Name: "system-bootstrap-node-bootstrapper",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:     "Group",
				Name:     "system:bootstrappers:nodes",
				APIGroup: rbacv1.GroupName,
			},
			{
				Kind:     "Group",
				Name:     "system:nodes",
				APIGroup: rbacv1.GroupName,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     "system:node-bootstrapper",
			APIGroup: rbacv1.GroupName,
		},
	}
}

// CSRApproverRoleBindingTemplate returns the CSR approver role binding template.
func CSRApproverRoleBindingTemplate() runtime.Object {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: v1.TypeMeta{
			Kind:       "ClusterRoleBinding",
			APIVersion: rbacv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: v1.ObjectMeta{
			Name: "system-bootstrap-approve-node-client-csr",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:     "Group",
				Name:     "system:bootstrappers:nodes",
				APIGroup: rbacv1.GroupName,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     "system:certificates.k8s.io:certificatesigningrequests:nodeclient",
			APIGroup: rbacv1.GroupName,
		},
	}
}

// CSRRenewalRoleBindingTemplate returns the CSR renewal role binding template.
func CSRRenewalRoleBindingTemplate() runtime.Object {
	return &rbacv1.ClusterRoleBinding{
		TypeMeta: v1.TypeMeta{
			Kind:       "ClusterRoleBinding",
			APIVersion: rbacv1.SchemeGroupVersion.String(),
		},
		ObjectMeta: v1.ObjectMeta{
			Name: "system-bootstrap-node-renewal",
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:     "Group",
				Name:     "system:nodes",
				APIGroup: rbacv1.GroupName,
			},
		},
		RoleRef: rbacv1.RoleRef{
			Kind:     "ClusterRole",
			Name:     "system:certificates.k8s.io:certificatesigningrequests:selfnodeclient",
			APIGroup: rbacv1.GroupName,
		},
	}
}
