/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package reg

import (
	"context"
	"log"
	"path"
	"strings"

	"github.com/containerd/containerd"
	"github.com/containerd/containerd/defaults"
	"github.com/containerd/containerd/namespaces"
)

type containerProc struct {
	Name   string // Friendly name
	ID     string // container sha/id
	Digest string // Container Digest
	Status string // Running state of container
	Pid    uint32

	Container containerd.Container
	Context   context.Context
}

func connect(namespace string) (*containerd.Client, context.Context, error) {
	client, err := containerd.New(defaults.DefaultAddress)
	return client, namespaces.WithNamespace(context.Background(), namespace), err
}

func containerID(namespace string) ([]containerProc, error) {
	client, ctx, err := connect(namespace)
	if err != nil {
		return nil, err
	}
	// nolint: errcheck
	defer client.Close()

	containers, err := client.Containers(ctx)
	if err != nil {
		return nil, err
	}

	cps := make([]containerProc, len(containers))

	for _, container := range containers {
		cp := containerProc{}

		info, err := container.Info(ctx)
		if err != nil {
			log.Println(err)
			continue
		}

		img, err := container.Image(ctx)
		if err != nil {
			log.Println(err)
			continue
		}

		task, err := container.Task(ctx, nil)
		if err != nil {
			log.Println(err)
			continue
		}

		status, err := task.Status(ctx)
		if err != nil {
			log.Println(err)
			continue
		}

		cp.ID = container.ID()
		cp.Name = container.ID()
		cp.Digest = img.Target().Digest.String()
		cp.Container = container
		cp.Context = ctx
		cp.Pid = task.Pid()
		cp.Status = strings.ToUpper(string(status.Status))

		if _, ok := info.Labels["io.kubernetes.pod.name"]; ok {
			cp.Name = path.Join(info.Labels["io.kubernetes.pod.namespace"], info.Labels["io.kubernetes.pod.name"])
		}

		cps = append(cps, cp)
	}

	return cps, nil
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
		imageList[image.Target().Digest.String()] = image.Name()
	}
	return imageList, nil
}
