// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package cri implements containers.Inspector via CRI
package cri

import (
	"context"
	"encoding/json"
	"strings"
	"syscall"
	"time"

	runtimeapi "k8s.io/cri-api/pkg/apis/runtime/v1alpha2"

	ctrs "github.com/talos-systems/talos/internal/pkg/containers"
	criclient "github.com/talos-systems/talos/internal/pkg/cri"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

type inspector struct {
	client *criclient.Client
	ctx    context.Context
}

type inspectorOptions struct {
	criEndpoint string
}

// Option configures containerd Inspector.
type Option func(*inspectorOptions)

// WithCRIEndpoint configures CRI endpoint to use.
func WithCRIEndpoint(endpoint string) Option {
	return func(o *inspectorOptions) {
		o.criEndpoint = endpoint
	}
}

// NewInspector builds new Inspector instance for CRI.
func NewInspector(ctx context.Context, options ...Option) (ctrs.Inspector, error) {
	var err error

	opt := inspectorOptions{
		criEndpoint: "unix:" + constants.ContainerdAddress,
	}

	for _, o := range options {
		o(&opt)
	}

	i := inspector{
		ctx: ctx,
	}

	i.client, err = criclient.NewClient(opt.criEndpoint, 10*time.Second)
	if err != nil {
		return nil, err
	}

	return &i, nil
}

// Close frees associated resources.
func (i *inspector) Close() error {
	return i.client.Close()
}

// Images returns a hash of image digest -> name.
func (i *inspector) Images() (map[string]string, error) {
	images, err := i.client.ListImages(i.ctx, &runtimeapi.ImageFilter{})
	if err != nil {
		return nil, err
	}

	result := make(map[string]string)

	for _, image := range images {
		if len(image.RepoTags) > 0 {
			result[image.Id] = image.RepoTags[0]
		}
	}

	return result, nil
}

func parseContainerDisplay(id string) (namespace, pod, name string) {
	slashIdx := strings.Index(id, "/")
	if slashIdx > 0 {
		namespace, pod = id[:slashIdx], id[slashIdx+1:]
		semicolonIdx := strings.LastIndex(pod, ":")

		if semicolonIdx > 0 {
			name = pod[semicolonIdx+1:]
			pod = pod[:semicolonIdx]
		}
	} else {
		name = id
	}

	return
}

// Container returns info about a single container.
//
// If container is not found, Container returns nil.
func (i *inspector) Container(id string) (*ctrs.Container, error) {
	namespace, pod, name := parseContainerDisplay(id)
	if pod == "" {
		return nil, nil
	}

	if name == "" { // request for a pod sandbox
		sandboxes, err := i.client.ListPodSandbox(i.ctx, &runtimeapi.PodSandboxFilter{
			State: &runtimeapi.PodSandboxStateValue{
				State: runtimeapi.PodSandboxState_SANDBOX_READY,
			},
			LabelSelector: map[string]string{
				"io.kubernetes.pod.name":      pod,
				"io.kubernetes.pod.namespace": namespace,
			},
		})
		if err != nil {
			return nil, err
		}

		if len(sandboxes) == 0 {
			return nil, nil
		}

		pod, err := i.buildPod(sandboxes[0])
		if err != nil {
			return nil, err
		}

		return pod.Containers[0], nil
	}

	// request for a container
	containers, err := i.client.ListContainers(i.ctx, &runtimeapi.ContainerFilter{
		LabelSelector: map[string]string{
			"io.kubernetes.pod.name":       pod,
			"io.kubernetes.pod.namespace":  namespace,
			"io.kubernetes.container.name": name,
		},
	})
	if err != nil {
		return nil, err
	}

	if len(containers) == 0 {
		return nil, nil
	}

	return i.buildContainer(containers[0])
}

func (i *inspector) buildPod(sandbox *runtimeapi.PodSandbox) (*ctrs.Pod, error) {
	sandboxStatus, sandboxInfo, err := i.client.PodSandboxStatus(i.ctx, sandbox.Id)
	if err != nil {
		return nil, err
	}

	podName := sandbox.Metadata.Namespace + "/" + sandbox.Metadata.Name
	pod := &ctrs.Pod{
		Name: podName,
		Containers: []*ctrs.Container{
			{
				Inspector:    i,
				Display:      podName,
				Name:         podName,
				ID:           sandbox.Id,
				PodName:      podName,
				Status:       sandboxStatus.State.String(),
				IsPodSandbox: true,
				Metrics:      &ctrs.ContainerMetrics{
					// assume pod sandbox uses zero
				},
			},
		},
	}

	if info, ok := sandboxInfo["info"]; ok {
		var verboseInfo map[string]interface{}

		if err := json.Unmarshal([]byte(info), &verboseInfo); err == nil {
			if pid, ok := verboseInfo["pid"]; ok {
				if fpid, ok := pid.(float64); ok {
					pod.Containers[0].Pid = uint32(fpid)
				}
			}

			if image, ok := verboseInfo["image"]; ok {
				if digest, ok := image.(string); ok {
					pod.Containers[0].Image = digest
					pod.Containers[0].Digest = digest
				}
			}
		}
	}

	return pod, nil
}

func (i *inspector) buildContainer(container *runtimeapi.Container) (*ctrs.Container, error) {
	containerStatus, containerInfo, err := i.client.ContainerStatus(i.ctx, container.Id, true)
	if err != nil {
		return nil, err
	}

	podName := container.Labels["io.kubernetes.pod.namespace"] + "/" + container.Labels["io.kubernetes.pod.name"]
	ctr := &ctrs.Container{
		Inspector:    i,
		Display:      podName + ":" + container.Metadata.Name,
		Name:         container.Metadata.Name,
		ID:           container.Id,
		Digest:       container.ImageRef,
		Image:        container.ImageRef,
		PodName:      podName,
		RestartCount: container.Annotations["io.kubernetes.container.restartCount"],
		Status:       container.State.String(),
		LogPath:      containerStatus.LogPath,
	}

	if info, ok := containerInfo["info"]; ok {
		var verboseInfo map[string]interface{}

		if err := json.Unmarshal([]byte(info), &verboseInfo); err == nil {
			if pid, ok := verboseInfo["pid"]; ok {
				if fpid, ok := pid.(float64); ok {
					ctr.Pid = uint32(fpid)
				}
			}
		}
	}

	return ctr, nil
}

// Pods collects information about running pods & containers.
//
//nolint:gocyclo
func (i *inspector) Pods() ([]*ctrs.Pod, error) {
	sandboxes, err := i.client.ListPodSandbox(i.ctx, &runtimeapi.PodSandboxFilter{
		State: &runtimeapi.PodSandboxStateValue{
			State: runtimeapi.PodSandboxState_SANDBOX_READY,
		},
	})
	if err != nil {
		return nil, err
	}

	containers, err := i.client.ListContainers(i.ctx, &runtimeapi.ContainerFilter{})
	if err != nil {
		return nil, err
	}

	metrics, err := i.client.ListContainerStats(i.ctx, &runtimeapi.ContainerStatsFilter{})
	if err != nil {
		return nil, err
	}

	metricsPerContainer := map[string]*runtimeapi.ContainerStats{}

	for _, metric := range metrics {
		metricsPerContainer[metric.Attributes.Id] = metric
	}

	images, err := i.Images()
	if err != nil {
		return nil, err
	}

	result := []*ctrs.Pod(nil)
	podMap := make(map[string]*ctrs.Pod)

	for _, sandbox := range sandboxes {
		pod, err := i.buildPod(sandbox)
		if err != nil {
			return nil, err
		}

		if imageName, ok := images[pod.Containers[0].Digest]; ok {
			pod.Containers[0].Image = imageName
		}

		result = append(result, pod)
		podMap[sandbox.Id] = pod
	}

	for _, container := range containers {
		pod := podMap[container.PodSandboxId]
		if pod == nil {
			// should never happen
			continue
		}

		ctr, err := i.buildContainer(container)
		if err != nil {
			return nil, err
		}

		if imageName, ok := images[ctr.Digest]; ok {
			ctr.Image = imageName
		}

		if metrics := metricsPerContainer[ctr.ID]; metrics != nil {
			ctr.Metrics = &ctrs.ContainerMetrics{}

			if metrics.Memory != nil && metrics.Memory.WorkingSetBytes != nil {
				ctr.Metrics.MemoryUsage = metrics.Memory.WorkingSetBytes.Value
			}

			if metrics.Cpu != nil && metrics.Cpu.UsageCoreNanoSeconds != nil {
				ctr.Metrics.CPUUsage = metrics.Cpu.UsageCoreNanoSeconds.Value
			}
		}

		pod.Containers = append(pod.Containers, ctr)
	}

	return result, nil
}

// GetProcessStderr returns process stderr.
func (i *inspector) GetProcessStderr(id string) (string, error) {
	// CRI doesn't seem to have an easy way to do that
	return "", nil
}

// Kill sends signal to container task.
func (i *inspector) Kill(id string, isPodSandbox bool, signal syscall.Signal) error {
	if isPodSandbox {
		return i.client.StopPodSandbox(i.ctx, id)
	}

	return i.client.StopContainer(i.ctx, id, 10)
}
