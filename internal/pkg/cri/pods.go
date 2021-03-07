// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package cri

import (
	"context"
	"fmt"
	"log"

	"golang.org/x/sync/errgroup"
	"google.golang.org/grpc/codes"
	grpcstatus "google.golang.org/grpc/status"
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

// StopAction for StopAndRemovePodSandboxes.
type StopAction int

// Stop actions.
const (
	StopOnly StopAction = iota
	StopAndRemove
)

// StopAndRemovePodSandboxes stops and removes all pods with the specified network mode. If no
// network mode is specified, all pods will be removed.
func (c *Client) StopAndRemovePodSandboxes(ctx context.Context, stopAction StopAction, modes ...runtimeapi.NamespaceMode) (err error) {
	pods, err := c.ListPodSandbox(ctx, nil)
	if err != nil {
		return err
	}

	var g errgroup.Group

	for _, pod := range pods {
		pod := pod // https://golang.org/doc/faq#closures_and_goroutines

		g.Go(func() error {
			status, _, e := c.PodSandboxStatus(ctx, pod.GetId())
			if e != nil {
				if grpcstatus.Code(e) == codes.NotFound {
					return nil
				}

				return e
			}

			networkMode := status.GetLinux().GetNamespaces().GetOptions().GetNetwork()

			// If any modes are specified, we verify that the current pod is
			// running any one of the modes. If it doesn't, we skip it.
			if len(modes) > 0 && !contains(networkMode, modes) {
				return nil
			}

			if pod.GetState() != runtimeapi.PodSandboxState_SANDBOX_READY {
				log.Printf("skipping pod %s/%s, state %s", pod.Metadata.Namespace, pod.Metadata.Name, pod.GetState())

				return nil
			}

			if e = stopAndRemove(ctx, stopAction, c, pod, networkMode.String()); e != nil {
				return fmt.Errorf("failed stopping pod %s/%s: %w", pod.Metadata.Namespace, pod.Metadata.Name, e)
			}

			return nil
		})
	}

	return g.Wait()
}

func contains(mode runtimeapi.NamespaceMode, modes []runtimeapi.NamespaceMode) bool {
	for _, m := range modes {
		if mode == m {
			return true
		}
	}

	return false
}

//nolint:gocyclo
func stopAndRemove(ctx context.Context, stopAction StopAction, client *Client, pod *runtimeapi.PodSandbox, mode string) (err error) {
	action := "stopping"
	status := "stopped"

	if stopAction == StopAndRemove {
		action = "removing"
		status = "removed"
	}

	log.Printf("%s pod %s/%s with network mode %q", action, pod.Metadata.Namespace, pod.Metadata.Name, mode)

	filter := &runtimeapi.ContainerFilter{
		PodSandboxId: pod.Id,
	}

	containers, err := client.ListContainers(ctx, filter)
	if err != nil {
		if grpcstatus.Code(err) == codes.NotFound {
			return nil
		}

		return err
	}

	var g errgroup.Group

	for _, container := range containers {
		container := container // https://golang.org/doc/faq#closures_and_goroutines

		g.Go(func() error {
			log.Printf("%s container %s/%s:%s", action, pod.Metadata.Namespace, pod.Metadata.Name, container.Metadata.Name)

			// TODO(andrewrynhard): Can we set the timeout dynamically?
			if err = client.StopContainer(ctx, container.Id, 30); err != nil {
				if grpcstatus.Code(err) == codes.NotFound {
					return nil
				}

				return err
			}

			if stopAction == StopAndRemove {
				if err = client.RemoveContainer(ctx, container.Id); err != nil {
					if grpcstatus.Code(err) == codes.NotFound {
						return nil
					}

					return err
				}
			}

			log.Printf("%s container %s/%s:%s", status, pod.Metadata.Namespace, pod.Metadata.Name, container.Metadata.Name)

			return nil
		})
	}

	if err = g.Wait(); err != nil {
		return err
	}

	if err = client.StopPodSandbox(ctx, pod.Id); err != nil {
		if grpcstatus.Code(err) == codes.NotFound {
			return nil
		}

		log.Printf("error stopping pod %s/%s, ignored: %s", pod.Metadata.Namespace, pod.Metadata.Name, err)

		return nil
	}

	if stopAction == StopAndRemove {
		if err = client.RemovePodSandbox(ctx, pod.Id); err != nil {
			if grpcstatus.Code(err) == codes.NotFound {
				return nil
			}

			log.Printf("error removing pod %s/%s, ignored: %s", pod.Metadata.Namespace, pod.Metadata.Name, err)

			return nil
		}
	}

	log.Printf("%s pod %s/%s", status, pod.Metadata.Namespace, pod.Metadata.Name)

	return nil
}
