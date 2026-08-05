// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package image

import (
	"context"
	"io"
	"log"
	"time"

	"github.com/siderolabs/talos/pkg/machinery/resources/cri"
)

// Image pull retry settings.
const (
	PullTimeout       = 20 * time.Minute
	PullRetryInterval = 5 * time.Second
)

// PullOption is an option for Pull function.
type PullOption func(*PullOptions)

// PullOptions configure Pull function.
type PullOptions struct {
	SkipIfAlreadyPulled bool
	MaxNotFoundRetries  int
	NewProgressReporter NewProgressReporter
	LogWriter           io.Writer
}

// DefaultPullOptions returns default options for Pull function.
func DefaultPullOptions() PullOptions {
	return PullOptions{
		SkipIfAlreadyPulled: false,
		MaxNotFoundRetries:  5,
		LogWriter:           log.Writer(),
	}
}

// WithSkipIfAlreadyPulled skips pulling if image is already pulled and unpacked.
func WithSkipIfAlreadyPulled() PullOption {
	return func(opts *PullOptions) {
		opts.SkipIfAlreadyPulled = true
	}
}

// WithMaxNotFoundRetries sets the maximum number of retries for not found errors.
//
// This option is only honored in the PullWithRetriesAndTimeout function,
// the Pull function will ignore this option and will not retry on not found errors.
func WithMaxNotFoundRetries(maxRetries int) PullOption {
	return func(opts *PullOptions) {
		opts.MaxNotFoundRetries = maxRetries
	}
}

// WithProgressReporter enables reporting pull progress.
func WithProgressReporter(newReporter NewProgressReporter) PullOption {
	return func(opts *PullOptions) {
		opts.NewProgressReporter = newReporter
	}
}

// WithLogWriter sets the writer for internal containerd logs.
func WithLogWriter(logWriter io.Writer) PullOption {
	return func(opts *PullOptions) {
		if logWriter == nil {
			logWriter = io.Discard
		}

		opts.LogWriter = logWriter
	}
}

// RegistriesBuilder is a function that returns registries configuration.
type RegistriesBuilder = func(context.Context) (cri.Registries, error)
