// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package containers provides the container implementatiom.
package containers

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/siderolabs/go-tail"

	"github.com/siderolabs/talos/pkg/chunker"
	"github.com/siderolabs/talos/pkg/chunker/file"
	"github.com/siderolabs/talos/pkg/chunker/stream"
)

// Container presents information about a container.
type Container struct {
	Inspector Inspector

	Display          string // Friendly Name
	Name             string // container name
	ID               string // container sha/id
	UID              string // container uid
	Digest           string // Container Digest
	Image            string
	PodName          string
	Sandbox          string
	Status           string // Running state of container
	RestartCount     string
	LogPath          string
	Metrics          *ContainerMetrics
	Pid              uint32
	IsPodSandbox     bool // real container or just pod sandbox
	NetworkNamespace string
}

// ContainerMetrics represents container cgroup stats.
type ContainerMetrics struct {
	MemoryUsage uint64
	CPUUsage    uint64
}

// GetProcessStderr returns process stderr.
func (c *Container) GetProcessStderr() (string, error) {
	return c.Inspector.GetProcessStderr(c.ID)
}

// GetLogFile returns path to log file, k8s-style.
func (c *Container) GetLogFile() string {
	if c.LogPath != "" {
		return c.LogPath
	}

	if c.Sandbox == "" || !strings.Contains(c.Display, ":") {
		return ""
	}

	return filepath.Join(c.Sandbox, c.Name, c.RestartCount+".log")
}

// Kill sends signal to container task.
func (c *Container) Kill(signal syscall.Signal) error {
	return c.Inspector.Kill(c.ID, c.IsPodSandbox, signal)
}

// GetLogChunker returns chunker for container log file.
func (c *Container) GetLogChunker(ctx context.Context, follow bool, tailLines int) (chunker.Chunker, io.Closer, error) {
	logFile := c.GetLogFile()
	if logFile != "" {
		f, err := os.OpenFile(logFile, os.O_RDONLY, 0)
		if err != nil {
			return nil, nil, err
		}

		if tailLines >= 0 {
			err = tail.SeekLines(f, tailLines)
			if err != nil {
				f.Close() //nolint:errcheck

				return nil, nil, fmt.Errorf("error tailing log: %w", err)
			}
		}

		var chunkerOptions []file.Option

		if follow {
			chunkerOptions = append(chunkerOptions, file.WithFollow())
		}

		return file.NewChunker(ctx, f, chunkerOptions...), f, nil
	}

	filename, err := c.GetProcessStderr()
	if err != nil {
		return nil, nil, err
	}

	if filename == "" {
		return nil, nil, errors.New("no log available")
	}

	f, err := os.OpenFile(filename, os.O_RDONLY, 0)
	if err != nil {
		return nil, nil, err
	}

	return stream.NewChunker(ctx, f), f, nil
}
