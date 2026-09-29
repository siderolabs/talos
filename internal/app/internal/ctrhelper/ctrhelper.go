// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package ctrhelper provides helpers for container-related APIs.
package ctrhelper

import (
	"context"

	containerdapi "github.com/containerd/containerd/v2/client"
	"github.com/containerd/containerd/v2/pkg/namespaces"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"github.com/siderolabs/talos/pkg/machinery/api/common"
	"github.com/siderolabs/talos/pkg/machinery/constants"
)

// ContainerdInstanceHelper helps to create containerd client and context from the given ContainerdInstance.
//
// This function returns:
//   - inbound context annotated with the appropriate containerd namespace
//   - detached (context.Background()) context with the appropriate containerd namespace
//   - containerd client
func ContainerdInstanceHelper(ctx context.Context, req *common.ContainerdInstance) (context.Context, context.Context, *containerdapi.Client, error) {
	containerdAddress, err := ContainerdInstanceAddress(req.GetDriver(), req.GetNamespace())
	if err != nil {
		return nil, nil, nil, err
	}

	containerdNamespace, err := ContainerdInstanceNamespace(req.GetNamespace())
	if err != nil {
		return nil, nil, nil, err
	}

	if req.GetDriver() == common.ContainerDriver_CONTAINERD && req.GetNamespace() == common.ContainerdNamespace_NS_CRI {
		return nil, nil, nil, status.Errorf(codes.InvalidArgument, "cannot use CRI namespace with containerd driver")
	}

	client, err := containerdapi.New(containerdAddress)
	if err != nil {
		return ctx, nil, nil, status.Errorf(codes.Unavailable, "error connecting to containerd: %s", err)
	}

	return namespaces.WithNamespace(ctx, containerdNamespace), namespaces.WithNamespace(context.Background(), containerdNamespace), client, nil
}

// ContainerdInstanceAddress resolves which containerd instance's socket a driver addresses.
func ContainerdInstanceAddress(driver common.ContainerDriver, namespace common.ContainerdNamespace) (string, error) {
	switch driver {
	case common.ContainerDriver_CONTAINERD:
		// The containerd instance backing every non-system namespace (taloscontainers included) is
		// the same one CRI uses; only the system namespace lives in Talos's own containerd instance.
		if namespace == common.ContainerdNamespace_NS_SYSTEM {
			return constants.SystemContainerdAddress, nil
		}

		return constants.CRIContainerdAddress, nil
	case common.ContainerDriver_CRI:
		return constants.CRIContainerdAddress, nil
	default:
		return "", status.Errorf(codes.InvalidArgument, "invalid containerd driver %s", driver)
	}
}

// ContainerdInstanceNamespace resolves the enum namespace selector to the raw containerd namespace name.
func ContainerdInstanceNamespace(namespace common.ContainerdNamespace) (string, error) {
	switch namespace {
	case common.ContainerdNamespace_NS_CRI:
		return constants.K8sContainerdNamespace, nil
	case common.ContainerdNamespace_NS_SYSTEM:
		return constants.SystemContainerdNamespace, nil
	case common.ContainerdNamespace_NS_TALOSCONTAINERS:
		return constants.TalosContainersContainerdNamespace, nil
	case common.ContainerdNamespace_NS_UNKNOWN:
		fallthrough
	default:
		return "", status.Errorf(codes.InvalidArgument, "invalid containerd namespace %s", namespace)
	}
}
