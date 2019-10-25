// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri

import (
	"context"
	"fmt"

	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"
)

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
		return "", fmt.Errorf("PodSandboxId is not set for sandbox %q", config.GetMetadata())
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
		return fmt.Errorf("StopPodSandbox %q from runtime service failed: %w", podSandBoxID, err)
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
		return fmt.Errorf("RemovePodSandbox %q from runtime service failed: %w", podSandBoxID, err)
	}

	return nil
}

// ListPodSandbox returns a list of PodSandboxes.
func (c *Client) ListPodSandbox(ctx context.Context, filter *runtimeapi.PodSandboxFilter) ([]*runtimeapi.PodSandbox, error) {
	resp, err := c.runtimeClient.ListPodSandbox(ctx, &runtimeapi.ListPodSandboxRequest{
		Filter: filter,
	})
	if err != nil {
		return nil, fmt.Errorf("ListPodSandbox with filter %+v from runtime service failed: %w", filter, err)
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
