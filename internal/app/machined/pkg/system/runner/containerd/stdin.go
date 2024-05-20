// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package containerd

import (
	"context"
	"io"

	containerd "github.com/containerd/containerd/v2/client"
)

// StdinCloser wraps io.Reader providing a signal when reader is read till EOF.
type StdinCloser struct {
	Stdin  io.Reader
	Closer chan struct{}
}

func (s *StdinCloser) Read(p []byte) (int, error) {
	n, err := s.Stdin.Read(p)
	if err == io.EOF {
		close(s.Closer)
	}

	return n, err
}

// WaitAndClose closes containerd task stdin when StdinCloser is exhausted.
func (s *StdinCloser) WaitAndClose(ctx context.Context, task containerd.Task) {
	select {
	case <-ctx.Done():
		return
	case <-s.Closer:
		//nolint:errcheck
		task.CloseIO(ctx, containerd.WithStdinCloser)
	}
}
