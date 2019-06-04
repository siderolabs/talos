/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package containers

import (
	"context"
	"path"
	"path/filepath"
	"strings"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/namespaces"
	"github.com/hashicorp/go-multierror"

	"github.com/talos-systems/talos/internal/pkg/constants"
)

// Inspector gather information about pods & containers
type Inspector struct {
	client *containerd.Client
	nsctx  context.Context
}

// NewInspector builds new Inspector instance in specified namespace
func NewInspector(ctx context.Context, namespace string) (*Inspector, error) {
	var err error

	i := Inspector{}
	i.client, err = containerd.New(constants.ContainerdAddress)
	if err != nil {
		return nil, err
	}
	i.nsctx = namespaces.WithNamespace(ctx, namespace)

	return &i, nil
}

// Close frees associated resources
func (i *Inspector) Close() error {
	return i.client.Close()
}

// Images returns a hash of image digest -> name
func (i *Inspector) Images() (map[string]string, error) {
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

// Pods collects information about running pods & containers
//
// nolint: gocyclo
func (i *Inspector) Pods() ([]*Pod, error) {
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
		pods     []*Pod
	)

	for _, cntr := range containers {
		cp := &Container{}

		info, err := cntr.Info(i.nsctx)
		if err != nil {
			multiErr = multierror.Append(multiErr, err)
			continue
		}

		spec, err := cntr.Spec(i.nsctx)
		if err != nil {
			multiErr = multierror.Append(multiErr, err)
			continue
		}

		img, err := cntr.Image(i.nsctx)
		if err != nil {
			multiErr = multierror.Append(multiErr, err)
			continue
		}

		task, err := cntr.Task(i.nsctx, nil)
		if err != nil {
			multiErr = multierror.Append(multiErr, err)
			continue
		}

		status, err := task.Status(i.nsctx)
		if err != nil {
			multiErr = multierror.Append(multiErr, err)
			continue
		}

		cp.inspector = i
		cp.ID = cntr.ID()
		cp.Name = cntr.ID()
		cp.Display = cntr.ID()
		cp.Digest = img.Target().Digest.String()
		cp.Image = imageList[img.Target().Digest.String()]
		cp.Pid = task.Pid()
		cp.Status = status

		if cname, ok := info.Labels["io.kubernetes.pod.name"]; ok {
			if cns, ok := info.Labels["io.kubernetes.pod.namespace"]; ok {
				cp.Display = path.Join(cns, cname)
			}
		}

		if status.Status == containerd.Running {
			metrics, err := task.Metrics(i.nsctx)
			if err != nil {
				multiErr = multierror.Append(multiErr, err)
			}
			cp.Metrics = metrics
		}

		// Save off an identifier for the pod
		// this is typically the container name (non-k8s namespace)
		// or will be k8s namespace"/"k8s pod name":"container name
		podName := cp.Display

		// Typically on actual application containers inside the pod/sandbox
		if _, ok := info.Labels["io.kubernetes.container.name"]; ok {
			cp.Name = info.Labels["io.kubernetes.container.name"]
			cp.Display = cp.Display + ":" + info.Labels["io.kubernetes.container.name"]
		}

		// Typically on the 'infrastructure' container, aka k8s.gcr.io/pause
		var sandbox string
		if _, ok := spec.Annotations["io.kubernetes.cri.sandbox-log-directory"]; ok {
			sandbox = spec.Annotations["io.kubernetes.cri.sandbox-log-directory"]
		}

		// Figure out if we need to create a new pod or append
		// to an existing pod
		// Also set pod sandbox ID if defined
		found := false
		for _, pod := range pods {
			if pod.Name != podName {
				continue
			}
			if sandbox != "" {
				pod.Sandbox = sandbox
			}
			pod.Containers = append(pod.Containers, cp)
			found = true
			break
		}

		if !found {
			p := &Pod{
				Name:       podName,
				Containers: []*Container{cp},
			}
			if sandbox != "" {
				p.Sandbox = sandbox
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
			if strings.Contains(cntr.Display, ":") && contents.Sandbox != "" {
				cntr.LogFile = filepath.Join(contents.Sandbox, cntr.Name, "0.log")
			}
		}
	}

	return pods, multiErr.ErrorOrNil()
}
