// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package console provides a console-based implementation of image pull progress reporting.
package console

import (
	"log"
	"sync"
	"time"

	"github.com/dustin/go-humanize"

	"github.com/siderolabs/talos/internal/pkg/containers/image"
	"github.com/siderolabs/talos/internal/pkg/containers/image/progress"
)

// ReportInterval is the interval between progress reports.
const ReportInterval = 15 * time.Second

type layerProgress struct {
	status progress.LayerPullStatus
	offset int64
	total  int64
}

// ProgressReporter reports image pull progress to the console.
type ProgressReporter struct {
	imageRef string

	mu     sync.Mutex
	layers map[string]*layerProgress
	stopCh chan struct{}
}

// NewProgressReporter creates a new ProgressReporter.
func NewProgressReporter(imageRef string) image.ProgressReporter {
	return &ProgressReporter{
		imageRef: imageRef,
	}
}

// Update implements UpdateFn interface.
func (c *ProgressReporter) Update(upd progress.LayerPullProgress) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.layers == nil {
		c.layers = make(map[string]*layerProgress)
	}

	lp, ok := c.layers[upd.LayerID]
	if !ok {
		lp = &layerProgress{}
		c.layers[upd.LayerID] = lp
	}

	if upd.Status == progress.LayerPullStatusDownloading {
		lp.total = upd.Total
		lp.offset = upd.Offset
	} else {
		lp.offset = lp.total
	}

	lp.status = upd.Status
}

// Start implements ProgressReporter interface.
func (c *ProgressReporter) Start() {
	c.stopCh = make(chan struct{})

	go func() {
		ticker := time.NewTicker(ReportInterval)
		defer ticker.Stop()

		c.reportProgress()

		for {
			select {
			case <-ticker.C:
				c.reportProgress()
			case <-c.stopCh:
				return
			}
		}
	}()
}

// Stop implements ProgressReporter interface.
func (c *ProgressReporter) Stop() {
	close(c.stopCh)
}

func (c *ProgressReporter) reportProgress() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.layers) == 0 {
		log.Printf("pulling image %s: starting...", c.imageRef)

		return
	}

	var (
		anyDownloading bool
		overallOffset  int64
		overallTotal   int64
	)

	for _, l := range c.layers {
		if l.status == progress.LayerPullStatusDownloading {
			anyDownloading = true
		}

		overallOffset += l.offset
		overallTotal += l.total
	}

	if !anyDownloading {
		log.Printf("pulling image %s: extracting...", c.imageRef)

		return
	}

	var percentage float64

	if overallTotal > 0 {
		percentage = float64(overallOffset) / float64(overallTotal) * 100.0
	}

	log.Printf("pulling image %s: downloading %d layers (%s/%s) (%.2f%%)...",
		c.imageRef, len(c.layers),
		humanize.IBytes(uint64(overallOffset)),
		humanize.IBytes(uint64(overallTotal)),
		percentage,
	)
}
