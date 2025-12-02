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
	var (
		containerdAddress   string
		containerdNamespace string
	)

	switch req.GetDriver() {
	case common.ContainerDriver_CONTAINERD:
		containerdAddress = constants.SystemContainerdAddress
	case common.ContainerDriver_CRI:
		containerdAddress = constants.CRIContainerdAddress
	default:
		return nil, nil, nil, status.Errorf(codes.InvalidArgument, "invalid containerd driver %s", req.GetDriver())
	}

	switch req.GetNamespace() {
	case common.ContainerdNamespace_NS_CRI:
		containerdNamespace = constants.K8sContainerdNamespace
	case common.ContainerdNamespace_NS_SYSTEM:
		containerdNamespace = constants.SystemContainerdNamespace
	case common.ContainerdNamespace_NS_UNKNOWN:
		fallthrough
	default:
		return nil, nil, nil, status.Errorf(codes.InvalidArgument, "invalid containerd namespace %s", req.GetNamespace())
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
