// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri

import (
	"context"
	"fmt"

	"github.com/cosi-project/runtime/pkg/controller"
	"github.com/cosi-project/runtime/pkg/resource"
	"github.com/cosi-project/runtime/pkg/safe"
	"github.com/cosi-project/runtime/pkg/state"

	"github.com/siderolabs/talos/pkg/machinery/resources/containers"
	"github.com/siderolabs/talos/pkg/machinery/resources/etcd"
	"github.com/siderolabs/talos/pkg/machinery/resources/k8s"
)

// KubernetesRefsToRetain preserves the images of the Kubernetes-related services Talos runs itself:
// etcd and the kubelet.
//
// Images of Kubernetes pods are deliberately out of scope, as the kubelet garbage collects those.
func KubernetesRefsToRetain(ctx context.Context, reader controller.Reader) ([]string, error) {
	var retain []string

	etcdSpec, err := safe.ReaderGet[*etcd.Spec](ctx, reader, resource.NewMetadata(etcd.NamespaceName, etcd.SpecType, etcd.SpecID, resource.VersionUndefined))
	if err != nil && !state.IsNotFoundError(err) {
		return nil, fmt.Errorf("error getting etcd spec: %w", err)
	}

	if etcdSpec != nil {
		retain = append(retain, etcdSpec.TypedSpec().Image)
	}

	kubeletSpec, err := safe.ReaderGet[*k8s.KubeletSpec](ctx, reader, resource.NewMetadata(k8s.NamespaceName, k8s.KubeletSpecType, k8s.KubeletID, resource.VersionUndefined))
	if err != nil && !state.IsNotFoundError(err) {
		return nil, fmt.Errorf("error getting kubelet spec: %w", err)
	}

	if kubeletSpec != nil {
		retain = append(retain, kubeletSpec.TypedSpec().Image)
	}

	return retain, nil
}

// TalosContainersRefsToRetain preserves the images of the containers declared in the machine
// configuration.
//
// It covers all three views a container has of its image, because they can disagree and each one
// alone leaves a window where an image in use looks collectable:
//
//   - ContainerSpec.Image.Ref is what is declared. It is a reference rather than a digest, so it
//     only resolves once the image is actually stored, and a moving tag resolves to whatever is
//     stored now rather than to what was pulled.
//   - ContainerImageStatus.Digest is what was pulled for that container, which pins the bytes a
//     moving tag resolved to at pull time.
//   - ContainerInstanceSpec.Image is the digest a running instance was created against. An instance
//     carries a resolved snapshot and is replaced rather than mutated, so after the reference is
//     edited this is the only place the still-running image is named.
func TalosContainersRefsToRetain(ctx context.Context, reader controller.Reader) ([]string, error) {
	var retain []string

	containerSpecs, err := safe.ReaderListAll[*containers.ContainerSpec](ctx, reader)
	if err != nil {
		return nil, fmt.Errorf("error listing container specs: %w", err)
	}

	for containerSpec := range containerSpecs.All() {
		retain = append(retain, containerSpec.TypedSpec().Image.Ref)
	}

	imageStatuses, err := safe.ReaderListAll[*containers.ContainerImageStatus](ctx, reader)
	if err != nil {
		return nil, fmt.Errorf("error listing container image statuses: %w", err)
	}

	for imageStatus := range imageStatuses.All() {
		// Empty until the pull completes; there is nothing to preserve before then.
		if digest := imageStatus.TypedSpec().Digest; digest != "" {
			retain = append(retain, digest)
		}
	}

	instanceSpecs, err := safe.ReaderListAll[*containers.ContainerInstanceSpec](ctx, reader)
	if err != nil {
		return nil, fmt.Errorf("error listing container instance specs: %w", err)
	}

	for instanceSpec := range instanceSpecs.All() {
		retain = append(retain, instanceSpec.TypedSpec().Image)
	}

	return retain, nil
}
