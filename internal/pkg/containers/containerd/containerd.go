// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package containerd implements containers.Inspector via containerd API
package containerd

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"
	"syscall"

	v1 "github.com/containerd/cgroups/stats/v1"
	"github.com/containerd/containerd"
	tasks "github.com/containerd/containerd/api/services/tasks/v1"
	"github.com/containerd/containerd/errdefs"
	"github.com/containerd/containerd/namespaces"
	"github.com/containerd/typeurl"
	"github.com/hashicorp/go-multierror"

	ctrs "github.com/talos-systems/talos/internal/pkg/containers"
	"github.com/talos-systems/talos/pkg/machinery/constants"
)

type inspector struct {
	client *containerd.Client
	nsctx  context.Context
}

type inspectorOptions struct {
	containerdAddress string
}

// Option configures containerd Inspector.
type Option func(*inspectorOptions)

// WithContainerdAddress configures containerd address to use.
func WithContainerdAddress(address string) Option {
	return func(o *inspectorOptions) {
		o.containerdAddress = address
	}
}

// NewInspector builds new Inspector instance in specified namespace.
func NewInspector(ctx context.Context, namespace string, options ...Option) (ctrs.Inspector, error) {
	var err error

	opt := inspectorOptions{
		containerdAddress: constants.ContainerdAddress,
	}

	for _, o := range options {
		o(&opt)
	}

	i := inspector{}

	i.client, err = containerd.New(opt.containerdAddress)
	if err != nil {
		return nil, err
	}

	i.nsctx = namespaces.WithNamespace(ctx, namespace)

	return &i, nil
}

// Close frees associated resources.
func (i *inspector) Close() error {
	return i.client.Close()
}

// Images returns a hash of image digest -> name.
func (i *inspector) Images() (map[string]string, error) {
	images, err := i.client.ListImages(i.nsctx, "")
	if err != nil {
		return nil, err
	}

	// create a map[sha]name for easier lookups later
	imageList := make(map[string]string, len(images))

	for _, image := range images {
		if strings.HasPrefix(image.Name(), "sha256:") {
			continue
		}

		imageList[image.Target().Digest.String()] = image.Name()
	}

	return imageList, nil
}

//nolint:gocyclo,cyclop
func (i *inspector) containerInfo(cntr containerd.Container, imageList map[string]string, singleLookup bool) (*ctrs.Container, error) {
	cp := &ctrs.Container{}

	info, err := cntr.Info(i.nsctx)
	if err != nil {
		return nil, fmt.Errorf("error getting container info for %q: %w", cntr.ID(), err)
	}

	spec, err := cntr.Spec(i.nsctx)
	if err != nil {
		return nil, fmt.Errorf("error getting container spec for %q: %w", cntr.ID(), err)
	}

	img, err := cntr.Image(i.nsctx)
	if err != nil {
		if !errdefs.IsNotFound(err) {
			return nil, fmt.Errorf("error getting container image for %q: %w", cntr.ID(), err)
		}
	}

	task, err := cntr.Task(i.nsctx, nil)
	if err != nil {
		if errdefs.IsNotFound(err) {
			// running task not found, skip container
			return nil, nil
		}

		return nil, fmt.Errorf("error getting container task for %q: %w", cntr.ID(), err)
	}

	status, err := task.Status(i.nsctx)
	if err != nil {
		return nil, fmt.Errorf("error getting task status for %q: %w", cntr.ID(), err)
	}

	cp.Inspector = i
	cp.ID = cntr.ID()
	cp.Name = cntr.ID()
	cp.Display = cntr.ID()
	cp.RestartCount = "0"

	if img != nil {
		cp.Digest = img.Target().Digest.String()
	}

	cp.Image = cp.Digest

	if imageList != nil {
		if resolved, ok := imageList[cp.Image]; ok {
			cp.Image = resolved
		}
	}

	cp.Pid = task.Pid()
	cp.Status = strings.ToUpper(string(status.Status))

	var (
		cname, cns string
		ok         bool
	)

	if cname, ok = info.Labels["io.kubernetes.pod.name"]; ok {
		if cns, ok = info.Labels["io.kubernetes.pod.namespace"]; ok {
			cp.Display = path.Join(cns, cname)
		}
	}

	if status.Status == containerd.Running {
		metrics, err := task.Metrics(i.nsctx)
		if err != nil {
			return nil, fmt.Errorf("error pulling metrics for %q: %w", cntr.ID(), err)
		}

		anydata, err := typeurl.UnmarshalAny(metrics.Data)
		if err != nil {
			return nil, fmt.Errorf("error unmarshalling metrics for %q: %w", cntr.ID(), err)
		}

		data, ok := anydata.(*v1.Metrics)
		if !ok {
			return nil, errors.New("failed to convert metric data to v1.Metrics")
		}

		cp.Metrics = &ctrs.ContainerMetrics{}

		mem := data.Memory
		if mem != nil && mem.Usage != nil {
			if mem.TotalInactiveFile < mem.Usage.Usage {
				cp.Metrics.MemoryUsage = mem.Usage.Usage - mem.TotalInactiveFile
			}
		}

		cpu := data.CPU
		if cpu != nil && cpu.Usage != nil {
			cp.Metrics.CPUUsage = cpu.Usage.Total
		}
	}

	// Save off an identifier for the pod
	// this is typically the container name (non-k8s namespace)
	// or will be k8s namespace"/"k8s pod name":"container name
	cp.PodName = cp.Display

	// Pull restart count
	// TODO: this doesn't work as CRI doesn't publish this to containerd annotations
	if _, ok := spec.Annotations["io.kubernetes.container.restartCount"]; ok {
		cp.RestartCount = spec.Annotations["io.kubernetes.container.restartCount"]
	}

	// Typically on the 'infrastructure' container, aka k8s.gcr.io/pause
	if _, ok := spec.Annotations["io.kubernetes.cri.sandbox-log-directory"]; ok {
		cp.Sandbox = spec.Annotations["io.kubernetes.cri.sandbox-log-directory"]
		cp.IsPodSandbox = true
	} else if singleLookup && cns != "" && cname != "" {
		// try to find matching infrastructure container and pull sandbox from it
		query := fmt.Sprintf("labels.\"io.kubernetes.pod.namespace\"==%q,labels.\"io.kubernetes.pod.name\"==%q", cns, cname)

		infraContainers, err := i.client.Containers(i.nsctx, query)
		if err == nil {
			for j := range infraContainers {
				if infraSpec, err := infraContainers[j].Spec(i.nsctx); err == nil {
					if spec.Annotations["io.kubernetes.sandbox-id"] != infraSpec.Annotations["io.kubernetes.sandbox-id"] {
						continue
					}

					if sandbox, found := infraSpec.Annotations["io.kubernetes.cri.sandbox-log-directory"]; found {
						cp.Sandbox = sandbox

						break
					}
				}
			}
		}
	}

	// Typically on actual application containers inside the pod/sandbox
	if _, ok := info.Labels["io.kubernetes.container.name"]; ok {
		cp.Name = info.Labels["io.kubernetes.container.name"]
		cp.Display = cp.Display + ":" + info.Labels["io.kubernetes.container.name"]
	}

	return cp, nil
}

// Container returns info about a single container.
//
// If container is not found, Container returns nil
//
//nolint:gocyclo
func (i *inspector) Container(id string) (*ctrs.Container, error) {
	var (
		query           string
		skipWithK8sName bool
	)

	// if id looks like k8s one, ns/pod:container, parse it and build query
	slashIdx := strings.Index(id, "/")
	if slashIdx > 0 {
		name := ""
		namespace, pod := id[:slashIdx], id[slashIdx+1:]

		semicolonIdx := strings.LastIndex(pod, ":")
		if semicolonIdx > 0 {
			name = pod[semicolonIdx+1:]
			pod = pod[:semicolonIdx]
		}

		query = fmt.Sprintf("labels.\"io.kubernetes.pod.namespace\"==%q,labels.\"io.kubernetes.pod.name\"==%q", namespace, pod)

		if name != "" {
			query += fmt.Sprintf(",labels.\"io.kubernetes.container.name\"==%q", name)
		} else {
			skipWithK8sName = true
		}
	} else {
		query = fmt.Sprintf("id==%q", id)
	}

	containers, err := i.client.Containers(i.nsctx, query)
	if err != nil {
		return nil, err
	}

	if len(containers) == 0 {
		return nil, nil
	}

	var cntr *ctrs.Container

	for j := range containers {
		if skipWithK8sName {
			var labels map[string]string

			if labels, err = containers[j].Labels(i.nsctx); err == nil {
				if _, found := labels["io.kubernetes.container.name"]; found {
					continue
				}
			}
		}

		cntr, err = i.containerInfo(containers[j], nil, true)
		if err == nil && cntr != nil {
			break
		}
	}

	return cntr, err
}

// Pods collects information about running pods & containers.
//
//nolint:gocyclo
func (i *inspector) Pods() ([]*ctrs.Pod, error) {
	imageList, err := i.Images()
	if err != nil {
		return nil, err
	}

	containers, err := i.client.Containers(i.nsctx)
	if err != nil {
		return nil, err
	}

	var (
		multiErr *multierror.Error
		pods     []*ctrs.Pod
	)

	for _, cntr := range containers {
		cp, err := i.containerInfo(cntr, imageList, false)
		if err != nil {
			multiErr = multierror.Append(multiErr, err)

			continue
		}

		if cp == nil {
			// not running container
			continue
		}

		// Figure out if we need to create a new pod or append
		// to an existing pod
		// Also set pod sandbox ID if defined
		found := false

		for _, pod := range pods {
			if pod.Name != cp.PodName {
				continue
			}

			if cp.Sandbox != "" {
				pod.Sandbox = cp.Sandbox
			}

			pod.Containers = append(pod.Containers, cp)
			found = true

			break
		}

		if !found {
			p := &ctrs.Pod{
				Name:       cp.PodName,
				Containers: []*ctrs.Container{cp},
				Sandbox:    cp.Sandbox,
			}
			pods = append(pods, p)
		}
	}

	// This seems janky because it is
	// But we need to loop through everything again to associate
	// the sandbox with the container name so we can get a proper
	// filepath to the location of the logfile
	for _, contents := range pods {
		for _, cntr := range contents.Containers {
			if cntr.Sandbox == "" && contents.Sandbox != "" {
				cntr.Sandbox = contents.Sandbox
			}
		}
	}

	return pods, multiErr.ErrorOrNil()
}

// GetProcessStderr returns process stderr.
func (i *inspector) GetProcessStderr(id string) (string, error) {
	task, err := i.client.TaskService().Get(i.nsctx, &tasks.GetRequest{ContainerID: id})
	if err != nil {
		return "", err
	}

	return task.Process.Stderr, nil
}

// Kill sends signal to container task.
func (i *inspector) Kill(id string, isPodSandbox bool, signal syscall.Signal) error {
	_, err := i.client.TaskService().Kill(i.nsctx, &tasks.KillRequest{ContainerID: id, Signal: uint32(signal)})

	return err
}
