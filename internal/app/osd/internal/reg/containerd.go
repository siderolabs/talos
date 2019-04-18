/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package reg

import (
	"context"
	"log"
	"path"
	"path/filepath"
	"strings"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/namespaces"
	"github.com/talos-systems/talos/internal/pkg/constants"
)

type container struct {
	Display string // Friendly Name
	Name    string // container name
	ID      string // container sha/id
	Digest  string // Container Digest
	Image   string
	Status  string // Running state of container
	Pid     uint32
	LogFile string
}

type pod struct {
	Name    string
	Sandbox string

	Containers []*container
}

func connect(namespace string) (*containerd.Client, context.Context, error) {
	client, err := containerd.New(constants.ContainerdAddress)
	return client, namespaces.WithNamespace(context.Background(), namespace), err
}

func podInfo(namespace string) ([]*pod, error) {
	pods := []*pod{}

	client, ctx, err := connect(namespace)
	if err != nil {
		return pods, err
	}
	// nolint: errcheck
	defer client.Close()

	var imageList map[string]string
	imageList, err = images(namespace)
	if err != nil {
		return pods, err
	}

	containers, err := client.Containers(ctx)
	if err != nil {
		return pods, err
	}

	for _, cntr := range containers {
		cp := &container{}

		info, err := cntr.Info(ctx)
		if err != nil {
			log.Println(err)
			continue
		}

		spec, err := cntr.Spec(ctx)
		if err != nil {
			log.Println(err)
			continue
		}

		img, err := cntr.Image(ctx)
		if err != nil {
			log.Println(err)
			continue
		}

		task, err := cntr.Task(ctx, nil)
		if err != nil {
			log.Println(err)
			continue
		}

		status, err := task.Status(ctx)
		if err != nil {
			log.Println(err)
			continue
		}

		cp.ID = cntr.ID()
		cp.Name = cntr.ID()
		cp.Display = cntr.ID()
		cp.Digest = img.Target().Digest.String()
		cp.Image = imageList[img.Target().Digest.String()]
		cp.Pid = task.Pid()
		cp.Status = strings.ToUpper(string(status.Status))

		if cname, ok := info.Labels["io.kubernetes.pod.name"]; ok {
			if cns, ok := info.Labels["io.kubernetes.pod.namespace"]; ok {
				cp.Display = path.Join(cns, cname)
			}
		}

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
			p := &pod{
				Name:       podName,
				Containers: []*container{cp},
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

	return pods, nil
}

func images(namespace string) (map[string]string, error) {
	client, ctx, err := connect(namespace)
	if err != nil {
		return nil, err
	}
	// nolint: errcheck
	defer client.Close()

	images, err := client.ListImages(ctx, "")
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
