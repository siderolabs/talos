// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package kubernetes

import (
	"context"
	"fmt"
	"net/netip"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/kubernetes"
)

// preserveServiceClusterIPs rewrites the clusterIP fields of Service objects to the
// values already allocated to them in the cluster.
//
// Service clusterIPs are immutable after creation, so applying a freshly generated
// manifest (e.g. the kube-dns Service during `talosctl upgrade-k8s`) must not attempt
// to change them: the generated object keeps everything else, but the allocated
// clusterIP(s) come from the live Service.
func preserveServiceClusterIPs(ctx context.Context, cluster UpgradeProvider, objects []*unstructured.Unstructured) error {
	serviceObjects := filterServiceObjects(objects)

	if len(serviceObjects) == 0 {
		return nil
	}

	config, err := cluster.K8sRestConfig(ctx)
	if err != nil {
		return err
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("error creating Kubernetes client: %w", err)
	}

	return preserveServiceClusterIPsWithClient(ctx, clientset, serviceObjects)
}

func filterServiceObjects(objects []*unstructured.Unstructured) []*unstructured.Unstructured {
	var serviceObjects []*unstructured.Unstructured

	for _, obj := range objects {
		if obj.GroupVersionKind().Group == "" && obj.GroupVersionKind().Kind == "Service" {
			serviceObjects = append(serviceObjects, obj)
		}
	}

	return serviceObjects
}

func preserveServiceClusterIPsWithClient(ctx context.Context, clientset kubernetes.Interface, objects []*unstructured.Unstructured) error {
	for _, obj := range objects {
		namespace, name := obj.GetNamespace(), obj.GetName()

		existing, err := clientset.CoreV1().Services(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			if apierrors.IsNotFound(err) {
				continue
			}

			return fmt.Errorf("error getting Service %s/%s: %w", namespace, name, err)
		}

		if existing.Spec.ClusterIP == "" && len(existing.Spec.ClusterIPs) == 0 {
			continue
		}

		if err := preserveServiceClusterIP(obj, existing); err != nil {
			return fmt.Errorf("error preserving clusterIP of Service %s/%s: %w", namespace, name, err)
		}
	}

	return nil
}

// preserveServiceClusterIP copies the allocated clusterIP fields from the live Service
// onto the generated object.
func preserveServiceClusterIP(obj *unstructured.Unstructured, existing *corev1.Service) error {
	clusterIPs := existing.Spec.ClusterIPs
	if len(clusterIPs) == 0 && existing.Spec.ClusterIP != "" {
		clusterIPs = []string{existing.Spec.ClusterIP}
	}

	if err := unstructured.SetNestedField(obj.Object, existing.Spec.ClusterIP, "spec", "clusterIP"); err != nil {
		return err
	}

	if err := unstructured.SetNestedStringSlice(obj.Object, clusterIPs, "spec", "clusterIPs"); err != nil {
		return err
	}

	ipFamilies := make([]string, 0, len(existing.Spec.IPFamilies))
	for _, family := range existing.Spec.IPFamilies {
		ipFamilies = append(ipFamilies, string(family))
	}

	if len(ipFamilies) == 0 && existing.Spec.ClusterIP != "" {
		if addr, err := netip.ParseAddr(existing.Spec.ClusterIP); err == nil && addr.Is4() {
			ipFamilies = []string{string(corev1.IPv4Protocol)}
		} else {
			ipFamilies = []string{string(corev1.IPv6Protocol)}
		}
	}

	if err := unstructured.SetNestedStringSlice(obj.Object, ipFamilies, "spec", "ipFamilies"); err != nil {
		return err
	}

	if existing.Spec.IPFamilyPolicy != nil {
		if err := unstructured.SetNestedField(obj.Object, string(*existing.Spec.IPFamilyPolicy), "spec", "ipFamilyPolicy"); err != nil {
			return err
		}
	}

	return nil
}
