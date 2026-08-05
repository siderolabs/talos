// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package image

import "github.com/siderolabs/talos/internal/pkg/containers/image/progress"

// ProgressReporter is an interface for reporting image pull progress.
type ProgressReporter interface {
	Start()
	Stop()
	Update(progress.LayerPullProgress)
}

// NewProgressReporter creates a new progress reporter.
type NewProgressReporter func(imageRef string) ProgressReporter

// NewSimpleProgressReporter creates a simple progress reporter that just needs Update function.
func NewSimpleProgressReporter(updateFn func(progress.LayerPullProgress)) NewProgressReporter {
	return func(imageRef string) ProgressReporter {
		return &simpleProgressReporter{
			updateFn: updateFn,
		}
	}
}

type simpleProgressReporter struct {
	updateFn func(progress.LayerPullProgress)
}

func (s *simpleProgressReporter) Start() {}

func (s *simpleProgressReporter) Stop() {}

func (s *simpleProgressReporter) Update(p progress.LayerPullProgress) {
	s.updateFn(p)
}
