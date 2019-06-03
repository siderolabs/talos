/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package containers

import (
	"syscall"

	"github.com/containerd/containerd"
	tasks "github.com/containerd/containerd/api/services/tasks/v1"
	"github.com/containerd/containerd/api/types"
)

// Container presents information about a container
type Container struct {
	inspector *Inspector

	Display string // Friendly Name
	Name    string // container name
	ID      string // container sha/id
	Digest  string // Container Digest
	Image   string
	Status  containerd.Status // Running state of container
	Pid     uint32
	LogFile string
	Metrics *types.Metric
}

// GetProcessStdout returns process stdout
func (c *Container) GetProcessStdout() (string, error) {
	task, err := c.inspector.client.TaskService().Get(c.inspector.nsctx, &tasks.GetRequest{ContainerID: c.ID})
	if err != nil {
		return "", err
	}

	return task.Process.Stdout, nil
}

// Kill sends signal to container task
func (c *Container) Kill(signal syscall.Signal) error {
	_, err := c.inspector.client.TaskService().Kill(c.inspector.nsctx, &tasks.KillRequest{ContainerID: c.ID, Signal: uint32(signal)})
	return err
}
