/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package cri

import (
	"context"
	"time"

	"github.com/pkg/errors"
	"google.golang.org/grpc"
	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
)

// Client is a lightweight implementation of CRI client.
//
// K8s version https://github.com/kubernetes/kubernetes/tree/master/pkg/kubelet/remote
// relies on k8s libs.
type Client struct {
	conn          *grpc.ClientConn
	runtimeClient runtimeapi.RuntimeServiceClient
	imagesClient  runtimeapi.ImageServiceClient
}

// maxMsgSize use 16MB as the default message size limit.
// grpc library default is 4MB
const maxMsgSize = 1024 * 1024 * 16

// NewClient builds CRI client
func NewClient(endpoint string, connectionTimeout time.Duration) (*Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), connectionTimeout)
	defer cancel()

	conn, err := grpc.DialContext(ctx, endpoint,
		grpc.WithInsecure(),
		grpc.WithBlock(),
		grpc.FailOnNonTempDialError(false),
		grpc.WithBackoffMaxDelay(3*time.Second),
		grpc.WithDefaultCallOptions(grpc.MaxCallRecvMsgSize(maxMsgSize)))
	if err != nil {
		return nil, errors.Wrapf(err, "error connecting to CRI")
	}

	return &Client{
		conn:          conn,
		runtimeClient: runtimeapi.NewRuntimeServiceClient(conn),
		imagesClient:  runtimeapi.NewImageServiceClient(conn),
	}, nil
}

// Close connection
func (c *Client) Close() error {
	return c.conn.Close()
}

// RunPodSandbox creates and starts a pod-level sandbox. Runtimes should ensure
// the sandbox is in ready state.
func (c *Client) RunPodSandbox(ctx context.Context, config *runtimeapi.PodSandboxConfig, runtimeHandler string) (string, error) {
	resp, err := c.runtimeClient.RunPodSandbox(ctx, &runtimeapi.RunPodSandboxRequest{
		Config:         config,
		RuntimeHandler: runtimeHandler,
	})
	if err != nil {
		return "", err
	}

	if resp.PodSandboxId == "" {
		return "", errors.Errorf("PodSandboxId is not set for sandbox %q", config.GetMetadata())
	}

	return resp.PodSandboxId, nil
}

// StopPodSandbox stops the sandbox. If there are any running containers in the
// sandbox, they should be forced to termination.
func (c *Client) StopPodSandbox(ctx context.Context, podSandBoxID string) error {
	_, err := c.runtimeClient.StopPodSandbox(ctx, &runtimeapi.StopPodSandboxRequest{
		PodSandboxId: podSandBoxID,
	})
	if err != nil {
		return errors.Wrapf(err, "StopPodSandbox %q from runtime service failed", podSandBoxID)
	}

	return nil
}

// RemovePodSandbox removes the sandbox. If there are any containers in the
// sandbox, they should be forcibly removed.
func (c *Client) RemovePodSandbox(ctx context.Context, podSandBoxID string) error {
	_, err := c.runtimeClient.RemovePodSandbox(ctx, &runtimeapi.RemovePodSandboxRequest{
		PodSandboxId: podSandBoxID,
	})
	if err != nil {
		return errors.Wrapf(err, "RemovePodSandbox %q from runtime service failed", podSandBoxID)
	}

	return nil
}

// ListPodSandbox returns a list of PodSandboxes.
func (c *Client) ListPodSandbox(ctx context.Context, filter *runtimeapi.PodSandboxFilter) ([]*runtimeapi.PodSandbox, error) {
	resp, err := c.runtimeClient.ListPodSandbox(ctx, &runtimeapi.ListPodSandboxRequest{
		Filter: filter,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "ListPodSandbox with filter %+v from runtime service failed", filter)
	}

	return resp.Items, nil
}

// PodSandboxStatus returns the status of the PodSandbox.
func (c *Client) PodSandboxStatus(ctx context.Context, podSandBoxID string) (*runtimeapi.PodSandboxStatus, map[string]string, error) {
	resp, err := c.runtimeClient.PodSandboxStatus(ctx, &runtimeapi.PodSandboxStatusRequest{
		PodSandboxId: podSandBoxID,
		Verbose:      true,
	})
	if err != nil {
		return nil, nil, err
	}

	return resp.Status, resp.Info, nil
}

// CreateContainer creates a new container in the specified PodSandbox.
func (c *Client) CreateContainer(ctx context.Context, podSandBoxID string, config *runtimeapi.ContainerConfig, sandboxConfig *runtimeapi.PodSandboxConfig) (string, error) {
	resp, err := c.runtimeClient.CreateContainer(ctx, &runtimeapi.CreateContainerRequest{
		PodSandboxId:  podSandBoxID,
		Config:        config,
		SandboxConfig: sandboxConfig,
	})
	if err != nil {
		return "", errors.Wrapf(err, "CreateContainer in sandbox %q from runtime service failed", podSandBoxID)
	}

	if resp.ContainerId == "" {
		return "", errors.Errorf("ContainerId is not set for container %q", config.GetMetadata())
	}

	return resp.ContainerId, nil
}

// StartContainer starts the container.
func (c *Client) StartContainer(ctx context.Context, containerID string) error {
	_, err := c.runtimeClient.StartContainer(ctx, &runtimeapi.StartContainerRequest{
		ContainerId: containerID,
	})
	if err != nil {
		return errors.Wrapf(err, "StartContainer %q from runtime service failed", containerID)
	}

	return nil
}

// StopContainer stops a running container with a grace period (i.e., timeout).
func (c *Client) StopContainer(ctx context.Context, containerID string, timeout int64) error {
	_, err := c.runtimeClient.StopContainer(ctx, &runtimeapi.StopContainerRequest{
		ContainerId: containerID,
		Timeout:     timeout,
	})
	if err != nil {
		return errors.Wrapf(err, "StopContainer %q from runtime service failed", containerID)
	}

	return nil
}

// RemoveContainer removes the container. If the container is running, the container
// should be forced to removal.
func (c *Client) RemoveContainer(ctx context.Context, containerID string) error {
	_, err := c.runtimeClient.RemoveContainer(ctx, &runtimeapi.RemoveContainerRequest{
		ContainerId: containerID,
	})
	if err != nil {
		return errors.Wrapf(err, "RemoveContainer %q from runtime service failed", containerID)
	}

	return nil
}

// ListContainers lists containers by filters.
func (c *Client) ListContainers(ctx context.Context, filter *runtimeapi.ContainerFilter) ([]*runtimeapi.Container, error) {
	resp, err := c.runtimeClient.ListContainers(ctx, &runtimeapi.ListContainersRequest{
		Filter: filter,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "ListContainers with filter %+v from runtime service failed", filter)
	}

	return resp.Containers, nil
}

// ContainerStatus returns the container status.
func (c *Client) ContainerStatus(ctx context.Context, containerID string) (*runtimeapi.ContainerStatus, map[string]string, error) {
	resp, err := c.runtimeClient.ContainerStatus(ctx, &runtimeapi.ContainerStatusRequest{
		ContainerId: containerID,
		Verbose:     true,
	})
	if err != nil {
		return nil, nil, errors.Wrapf(err, "ContainerStatus %q from runtime service failed", containerID)
	}

	return resp.Status, resp.Info, nil
}

// ContainerStats returns the stats of the container.
func (c *Client) ContainerStats(ctx context.Context, containerID string) (*runtimeapi.ContainerStats, error) {
	resp, err := c.runtimeClient.ContainerStats(ctx, &runtimeapi.ContainerStatsRequest{
		ContainerId: containerID,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "ContainerStatus %q from runtime service failed", containerID)
	}

	return resp.GetStats(), nil
}

// ListContainerStats returns stats for all the containers matching the filter
func (c *Client) ListContainerStats(ctx context.Context, filter *runtimeapi.ContainerStatsFilter) ([]*runtimeapi.ContainerStats, error) {
	resp, err := c.runtimeClient.ListContainerStats(ctx, &runtimeapi.ListContainerStatsRequest{
		Filter: filter,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "ListContainerStats with filter %+v from runtime service failed", filter)
	}

	return resp.GetStats(), nil
}

// PullImage pulls container image
func (c *Client) PullImage(ctx context.Context, image *runtimeapi.ImageSpec, sandboxConfig *runtimeapi.PodSandboxConfig) (string, error) {
	resp, err := c.imagesClient.PullImage(ctx, &runtimeapi.PullImageRequest{
		Image:         image,
		SandboxConfig: sandboxConfig,
	})
	if err != nil {
		return "", errors.Wrapf(err, "error pulling image %+v", image)
	}

	return resp.ImageRef, nil
}

// ListImages lists available images
func (c *Client) ListImages(ctx context.Context, filter *runtimeapi.ImageFilter) ([]*runtimeapi.Image, error) {
	resp, err := c.imagesClient.ListImages(ctx, &runtimeapi.ListImagesRequest{
		Filter: filter,
	})
	if err != nil {
		return nil, errors.Wrapf(err, "error listing imags")
	}

	return resp.Images, nil

}
