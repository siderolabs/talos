// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package progress provides functionality to track and report image pull progress.
package progress

import (
	"context"
	"errors"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/core/remotes"
	"github.com/containerd/containerd/v2/core/snapshots"
	"github.com/containerd/containerd/v2/pkg/snapshotters"
	"github.com/containerd/errdefs"
	"github.com/moby/moby/client/pkg/stringid"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

// LayerPullStatus represents the status of a single image layer during pull.
type LayerPullStatus int

// Possible values for LayerPullStatus.
const (
	LayerPullStatusDownloading LayerPullStatus = iota
	LayerPullStatusDownloadComplete
	LayerPullStatusExtracting
	LayerPullStatusExtractComplete
	LayerPullStatusAlreadyExists
)

// LayerPullProgress represents the progress of an image pull operation.
type LayerPullProgress struct {
	LayerID string
	Status  LayerPullStatus
	// If Status is Extracting, this shows the elapsed time since extraction started.
	Elapsed time.Duration
	// If Status is Downloading, these show the current offset and total size of the layer.
	Offset int64
	Total  int64
}

// UpdateFn is used by PullProgress to report progress updates.
type UpdateFn func(LayerPullProgress)

// PullProgress tracks and reports the progress of image pulls.
type PullProgress struct {
	store       content.Store
	snapshotter snapshots.Snapshotter
	updateFn    UpdateFn

	mu    sync.Mutex
	descs map[digest.Digest]ocispec.Descriptor // guarded by mu

	layers      []ocispec.Descriptor
	unpackStart map[digest.Digest]time.Time
}

// NewPullProgress creates a new PullProgress instance.
func NewPullProgress(store content.Store, snapshotter snapshots.Snapshotter, fn UpdateFn) *PullProgress {
	return &PullProgress{
		updateFn:    fn,
		store:       store,
		snapshotter: snapshotter,
		descs:       make(map[digest.Digest]ocispec.Descriptor),
		mu:          sync.Mutex{},
	}
}

// ShowProgress starts tracking and reporting pull progress.
func (p *PullProgress) ShowProgress(ctx context.Context) func() {
	ctx, cancel := context.WithCancel(ctx)

	start := time.Now()
	lastUpdate := make(chan struct{})

	go func() {
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), time.Millisecond*500)
				defer cancel()

				if err := p.trackProgress(ctx, start); err != nil {
					log.Printf("failed to write pull progress: %s", err.Error())
				}

				close(lastUpdate)

				return

			case <-ticker.C:
				if err := p.trackProgress(ctx, start); err != nil {
					if !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded) {
						log.Printf("Updating progress failed %s", err.Error())
					}
				}
			}
		}
	}()

	// call this when done pulling to stop progress updates
	return func() {
		cancel()
		<-lastUpdate
	}
}

func (p *PullProgress) trackProgress(ctx context.Context, start time.Time) error {
	err := p.trackOngoingPulls(ctx, start)
	if err != nil {
		return err
	}

	return p.trackPulledLayers(ctx)
}

func (p *PullProgress) trackOngoingPulls(ctx context.Context, start time.Time) error { //nolint:gocyclo
	actives, err := p.store.ListStatuses(ctx, "")
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}

		log.Printf("status check failed: %s", err.Error())

		return nil
	}

	pulling := make(map[string]content.Status, len(actives))

	for _, status := range actives {
		pulling[status.Ref] = status
	}

	for _, j := range p.jobs() {
		key := remotes.MakeRefKey(ctx, j)
		if info, ok := pulling[key]; ok {
			if info.Offset == 0 {
				continue
			}

			p.updateFn(LayerPullProgress{
				LayerID: stringid.TruncateID(j.Digest.Encoded()),
				Status:  LayerPullStatusDownloading,
				Offset:  info.Offset,
				Total:   info.Total,
			})

			continue
		}

		info, err := p.store.Info(ctx, j.Digest)
		switch {
		case err != nil:
			if !errdefs.IsNotFound(err) {
				return err
			}

		case info.CreatedAt.After(start):
			p.updateFn(LayerPullProgress{
				LayerID: stringid.TruncateID(j.Digest.Encoded()),
				Status:  LayerPullStatusDownloadComplete,
			})

			if images.IsLayerType(j.MediaType) {
				p.layers = append(p.layers, j)
			}

			p.remove(j)

		default:
			p.updateFn(LayerPullProgress{
				LayerID: stringid.TruncateID(j.Digest.Encoded()),
				Status:  LayerPullStatusAlreadyExists,
			})

			if images.IsLayerType(j.MediaType) {
				p.layers = append(p.layers, j)
			}

			p.remove(j)
		}
	}

	return nil
}

func (p *PullProgress) trackPulledLayers(ctx context.Context) error {
	var committedIdx []int

	for idx, desc := range p.layers {
		walkFilter := "labels.\"" + snapshotters.TargetLayerDigestLabel + "\"==" + p.layers[idx].Digest.String()

		err := p.snapshotter.Walk(ctx, func(ctx context.Context, sn snapshots.Info) error {
			if sn.Kind == snapshots.KindActive {
				if p.unpackStart == nil {
					p.unpackStart = make(map[digest.Digest]time.Time)
				}

				var elapsed time.Duration

				if began, ok := p.unpackStart[desc.Digest]; !ok {
					p.unpackStart[desc.Digest] = time.Now()
				} else {
					elapsed = time.Since(began)
				}

				p.updateFn(LayerPullProgress{
					LayerID: stringid.TruncateID(desc.Digest.Encoded()),
					Status:  LayerPullStatusExtracting,
					Elapsed: elapsed,
				})

				return nil
			}

			if sn.Kind == snapshots.KindCommitted {
				p.updateFn(
					LayerPullProgress{
						LayerID: stringid.TruncateID(desc.Digest.Encoded()),
						Status:  LayerPullStatusExtractComplete,
					},
				)

				committedIdx = append(committedIdx, idx)

				return nil
			}

			return nil
		}, walkFilter)
		if err != nil {
			return err
		}
	}

	// remove finished/committed layers from p.layers
	if len(committedIdx) > 0 {
		sort.Ints(committedIdx)

		for i := len(committedIdx) - 1; i >= 0; i-- {
			p.layers = append(p.layers[:committedIdx[i]], p.layers[committedIdx[i]+1:]...)
		}
	}

	return nil
}

// Add adds new descriptors to be tracked.
func (p *PullProgress) Add(desc ...ocispec.Descriptor) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, d := range desc {
		if _, ok := p.descs[d.Digest]; ok {
			continue
		}

		p.descs[d.Digest] = d
	}
}

func (p *PullProgress) remove(desc ocispec.Descriptor) {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.descs, desc.Digest)
}

func (p *PullProgress) jobs() []ocispec.Descriptor {
	p.mu.Lock()
	defer p.mu.Unlock()

	descs := make([]ocispec.Descriptor, 0, len(p.descs))
	for _, d := range p.descs {
		descs = append(descs, d)
	}

	return descs
}
