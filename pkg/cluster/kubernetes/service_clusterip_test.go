// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubernetes

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes/fake"
)

func TestPreserveServiceClusterIPsWithClient(t *testing.T) {
	dualStackPolicy := corev1.IPFamilyPolicyRequireDualStack
	singleStackPolicy := corev1.IPFamilyPolicySingleStack

	generated := func() *unstructured.Unstructured {
		return &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "v1",
			"kind":       "Service",
			"metadata": map[string]any{
				"name":      "kube-dns",
				"namespace": "kube-system",
			},
			"spec": map[string]any{
				"clusterIP":      "10.96.0.10",
				"clusterIPs":     []any{"10.96.0.10"},
				"ipFamilies":     []any{"IPv4"},
				"ipFamilyPolicy": "SingleStack",
				"selector":       map[string]any{"k8s-app": "kube-dns"},
			},
		}}
	}

	existing := func(clusterIP string, clusterIPs []string, families []corev1.IPFamily, policy *corev1.IPFamilyPolicy) *corev1.Service {
		return &corev1.Service{
			ObjectMeta: metav1.ObjectMeta{
				Name:      "kube-dns",
				Namespace: "kube-system",
			},
			Spec: corev1.ServiceSpec{
				ClusterIP:      clusterIP,
				ClusterIPs:     clusterIPs,
				IPFamilies:     families,
				IPFamilyPolicy: policy,
			},
		}
	}

	for _, tt := range []struct {
		name             string
		existing         *corev1.Service // nil → Service doesn't exist yet
		expectedIP       string
		expectedIPs      []string
		expectedFamilies []string
		expectedPolicy   string
	}{
		{
			name:             "service exists with different clusterIP",
			existing:         existing("10.96.0.42", []string{"10.96.0.42"}, []corev1.IPFamily{corev1.IPv4Protocol}, &singleStackPolicy),
			expectedIP:       "10.96.0.42",
			expectedIPs:      []string{"10.96.0.42"},
			expectedFamilies: []string{"IPv4"},
			expectedPolicy:   "SingleStack",
		},
		{
			name:             "service exists with same clusterIP",
			existing:         existing("10.96.0.10", []string{"10.96.0.10"}, []corev1.IPFamily{corev1.IPv4Protocol}, &singleStackPolicy),
			expectedIP:       "10.96.0.10",
			expectedIPs:      []string{"10.96.0.10"},
			expectedFamilies: []string{"IPv4"},
			expectedPolicy:   "SingleStack",
		},
		{
			name: "dual-stack service",
			existing: existing(
				"10.96.0.42",
				[]string{"10.96.0.42", "fd01::a"},
				[]corev1.IPFamily{corev1.IPv4Protocol, corev1.IPv6Protocol},
				&dualStackPolicy,
			),
			expectedIP:       "10.96.0.42",
			expectedIPs:      []string{"10.96.0.42", "fd01::a"},
			expectedFamilies: []string{"IPv4", "IPv6"},
			expectedPolicy:   "RequireDualStack",
		},
		{
			name:             "service does not exist yet",
			existing:         nil,
			expectedIP:       "10.96.0.10",
			expectedIPs:      []string{"10.96.0.10"},
			expectedFamilies: []string{"IPv4"},
			expectedPolicy:   "SingleStack",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			obj := generated()

			var clientset *fake.Clientset

			if tt.existing != nil {
				clientset = fake.NewSimpleClientset(tt.existing)
			} else {
				clientset = fake.NewSimpleClientset()
			}

			require.NoError(t, preserveServiceClusterIPsWithClient(context.Background(), clientset, []*unstructured.Unstructured{obj}))

			clusterIP, ok, err := unstructured.NestedString(obj.Object, "spec", "clusterIP")
			require.NoError(t, err)
			assert.True(t, ok)
			assert.Equal(t, tt.expectedIP, clusterIP)

			clusterIPs, ok, err := unstructured.NestedStringSlice(obj.Object, "spec", "clusterIPs")
			require.NoError(t, err)
			assert.True(t, ok)
			assert.Equal(t, tt.expectedIPs, clusterIPs)

			ipFamilies, ok, err := unstructured.NestedStringSlice(obj.Object, "spec", "ipFamilies")
			require.NoError(t, err)
			assert.True(t, ok)
			assert.Equal(t, tt.expectedFamilies, ipFamilies)

			ipFamilyPolicy, ok, err := unstructured.NestedString(obj.Object, "spec", "ipFamilyPolicy")
			require.NoError(t, err)
			assert.True(t, ok)
			assert.Equal(t, tt.expectedPolicy, ipFamilyPolicy)
		})
	}
}

func TestFilterServiceObjects(t *testing.T) {
	objects := []*unstructured.Unstructured{
		{
			Object: map[string]any{
				"apiVersion": "v1",
				"kind":       "Service",
			},
		},
		{
			Object: map[string]any{
				"apiVersion": "apps/v1",
				"kind":       "Deployment",
			},
		},
		{
			Object: map[string]any{
				"apiVersion": "v1",
				"kind":       "ConfigMap",
			},
		},
	}

	serviceObjects := filterServiceObjects(objects)

	assert.Len(t, serviceObjects, 1)
	assert.Equal(t, "Service", serviceObjects[0].GetKind())
}
