// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package md provides a Go interface to Linux MD (software RAID) arrays via
// the mdadm(8) utility.
package md

import (
	"context"
	"fmt"

	"github.com/siderolabs/go-cmd/pkg/cmd"
)

// MD provides methods for managing MD (software RAID) arrays.
type MD struct {
	mdadm string
}

// New creates a new MD instance, resolving the mdadm binary.
func New(opts ...Option) (*MD, error) {
	md := &MD{
		mdadm: "/sbin/mdadm",
	}

	for _, opt := range opts {
		opt(md)
	}

	return md, nil
}

// Option is a functional option for configuring the MD instance.
type Option func(*MD)

// WithMdadmPath sets an explicit path to the mdadm binary.
func WithMdadmPath(path string) Option {
	return func(md *MD) {
		md.mdadm = path
	}
}

// run executes `mdadm <args...>` and returns stdout.
//
// Errors are normalised through classifyError so every caller sees the same
// sentinel set (ErrNotFound, ErrInUse, ErrExists, ErrCommand). The raw mdadm
// stderr is kept out of the returned error chain - only the sentinel is
// wrapped - so it will not be surfaced to API clients by mistake.
func (md *MD) run(ctx context.Context, args ...string) (string, error) {
	out, err := cmd.RunWithOptions(ctx, md.mdadm, args, cmd.WithFullStdoutCapture())
	if err != nil {
		return "", fmt.Errorf("mdadm failed: %w", classifyError(err))
	}

	return out, nil
}
