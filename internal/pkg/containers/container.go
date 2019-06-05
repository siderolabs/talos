/* This Source Code Form is subject to the terms of the Mozilla Public
 * License, v. 2.0. If a copy of the MPL was not distributed with this
 * file, You can obtain one at http://mozilla.org/MPL/2.0/. */

package containers

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/containerd/containerd"
	tasks "github.com/containerd/containerd/api/services/tasks/v1"
	"github.com/containerd/containerd/api/types"

	"github.com/talos-systems/talos/internal/pkg/chunker"
	"github.com/talos-systems/talos/internal/pkg/chunker/file"
	"github.com/talos-systems/talos/internal/pkg/chunker/stream"
)

// Container presents information about a container
type Container struct {
	inspector *Inspector

	Display      string // Friendly Name
	Name         string // container name
	ID           string // container sha/id
	Digest       string // Container Digest
	Image        string
	PodName      string
	Sandbox      string
	Status       containerd.Status // Running state of container
	Pid          uint32
	RestartCount string
	Metrics      *types.Metric
}

// GetProcessStderr returns process stderr
func (c *Container) GetProcessStderr() (string, error) {
	task, err := c.inspector.client.TaskService().Get(c.inspector.nsctx, &tasks.GetRequest{ContainerID: c.ID})
	if err != nil {
		return "", err
	}

	return task.Process.Stderr, nil
}

// GetLogFile returns path to log file, k8s-style
func (c *Container) GetLogFile() string {
	if c.Sandbox == "" || !strings.Contains(c.Display, ":") {
		return ""
	}

	return filepath.Join(c.Sandbox, c.Name, c.RestartCount+".log")
}

// Kill sends signal to container task
func (c *Container) Kill(signal syscall.Signal) error {
	_, err := c.inspector.client.TaskService().Kill(c.inspector.nsctx, &tasks.KillRequest{ContainerID: c.ID, Signal: uint32(signal)})
	return err
}

// GetLogChunker returns chunker for container log file
func (c *Container) GetLogChunker() (chunker.Chunker, io.Closer, error) {
	logFile := c.GetLogFile()
	log.Printf("logFile = %q", logFile)
	if logFile != "" {
		f, err := os.OpenFile(logFile, os.O_RDONLY, 0)
		if err != nil {
			return nil, nil, err
		}

		return file.NewChunker(f), f, nil
	}

	filename, err := c.GetProcessStderr()
	if err != nil {
		return nil, nil, err
	}
	if filename == "" {
		return nil, nil, fmt.Errorf("no log available")
	}

	f, err := os.OpenFile(filename, os.O_RDONLY, 0)
	if err != nil {
		return nil, nil, err
	}

	return stream.NewChunker(f), f, nil
}
