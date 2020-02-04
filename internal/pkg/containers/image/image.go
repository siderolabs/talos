// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package image

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/remotes/docker"

	"github.com/talos-systems/talos/pkg/retry"
)

// Pull is a convenience function that wraps the containerd image pull func with
// retry functionality.
func Pull(ctx context.Context, client *containerd.Client, ref string) (img containerd.Image, err error) {
	var resolver = docker.NewResolver(docker.ResolverOptions{
		Hosts: func(host string) ([]docker.RegistryHost, error) {
			switch host {
			case "docker.io":
				return []docker.RegistryHost{
					{
						Client:       http.DefaultClient,
						Scheme:       "http",
						Host:         "172.20.0.1:5000",
						Path:         "/v2",
						Capabilities: docker.HostCapabilityResolve | docker.HostCapabilityPull,
					},
				}, nil
			case "k8s.gcr.io":
				return []docker.RegistryHost{
					{
						Client:       http.DefaultClient,
						Scheme:       "http",
						Host:         "172.20.0.1:5001",
						Path:         "/v2",
						Capabilities: docker.HostCapabilityResolve | docker.HostCapabilityPull,
					},
				}, nil
			case "quay.io":
				return []docker.RegistryHost{
					{
						Client:       http.DefaultClient,
						Scheme:       "http",
						Host:         "172.20.0.1:5002",
						Path:         "/v2",
						Capabilities: docker.HostCapabilityResolve | docker.HostCapabilityPull,
					},
				}, nil
			default:
				defaultHost, err := docker.DefaultHost(host)
				if err != nil {
					return nil, err
				}

				return []docker.RegistryHost{
					{
						Client:       http.DefaultClient,
						Scheme:       "https",
						Host:         defaultHost,
						Path:         "/v2",
						Capabilities: docker.HostCapabilityResolve | docker.HostCapabilityPull,
					},
				}, nil
			}
		},
	})

	err = retry.Exponential(1*time.Minute, retry.WithUnits(1*time.Second)).Retry(func() error {
		if img, err = client.Pull(ctx, ref, containerd.WithPullUnpack, containerd.WithResolver(resolver)); err != nil {
			return retry.ExpectedError(fmt.Errorf("failed to pull image %q: %w", ref, err))
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return img, nil
}
