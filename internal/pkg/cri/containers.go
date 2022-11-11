// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri

import (
	"context"
	"fmt"

	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1"
)

// CreateContainer creates a new container in the specified PodSandbox.
func (c *Client) CreateContainer(ctx context.Context, podSandBoxID string, config *runtimeapi.ContainerConfig, sandboxConfig *runtimeapi.PodSandboxConfig) (string, error) {
	resp, err := c.runtimeClient.CreateContainer(ctx, &runtimeapi.CreateContainerRequest{
		PodSandboxId:  podSandBoxID,
		Config:        config,
		SandboxConfig: sandboxConfig,
	})
	if err != nil {
		return "", fmt.Errorf("CreateContainer in sandbox %q from runtime service failed: %w", podSandBoxID, err)
	}

	if resp.ContainerId == "" {
		return "", fmt.Errorf("ContainerId is not set for container %q", config.GetMetadata())
	}

	return resp.ContainerId, nil
}

// StartContainer starts the container.
func (c *Client) StartContainer(ctx context.Context, containerID string) error {
	_, err := c.runtimeClient.StartContainer(ctx, &runtimeapi.StartContainerRequest{
		ContainerId: containerID,
	})
	if err != nil {
		return fmt.Errorf("StartContainer %q from runtime service failed: %w", containerID, err)
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
		return fmt.Errorf("StopContainer %q from runtime service failed: %w", containerID, err)
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
		return fmt.Errorf("RemoveContainer %q from runtime service failed: %w", containerID, err)
	}

	return nil
}

// ListContainers lists containers by filters.
func (c *Client) ListContainers(ctx context.Context, filter *runtimeapi.ContainerFilter) ([]*runtimeapi.Container, error) {
	resp, err := c.runtimeClient.ListContainers(ctx, &runtimeapi.ListContainersRequest{
		Filter: filter,
	})
	if err != nil {
		return nil, fmt.Errorf("ListContainers with filter %+v from runtime service failed: %w", filter, err)
	}

	return resp.Containers, nil
}

// ContainerStatus returns the container status.
func (c *Client) ContainerStatus(ctx context.Context, containerID string, verbose bool) (*runtimeapi.ContainerStatus, map[string]string, error) {
	resp, err := c.runtimeClient.ContainerStatus(ctx, &runtimeapi.ContainerStatusRequest{
		ContainerId: containerID,
		Verbose:     verbose,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("ContainerStatus %q from runtime service failed: %w", containerID, err)
	}

	return resp.Status, resp.Info, nil
}

// ContainerStats returns the stats of the container.
func (c *Client) ContainerStats(ctx context.Context, containerID string) (*runtimeapi.ContainerStats, error) {
	resp, err := c.runtimeClient.ContainerStats(ctx, &runtimeapi.ContainerStatsRequest{
		ContainerId: containerID,
	})
	if err != nil {
		return nil, fmt.Errorf("ContainerStatus %q from runtime service failed: %w", containerID, err)
	}

	return resp.GetStats(), nil
}

// ListContainerStats returns stats for all the containers matching the filter.
func (c *Client) ListContainerStats(ctx context.Context, filter *runtimeapi.ContainerStatsFilter) ([]*runtimeapi.ContainerStats, error) {
	resp, err := c.runtimeClient.ListContainerStats(ctx, &runtimeapi.ListContainerStatsRequest{
		Filter: filter,
	})
	if err != nil {
		return nil, fmt.Errorf("ListContainerStats with filter %+v from runtime service failed: %w", filter, err)
	}

	return resp.GetStats(), nil
}
