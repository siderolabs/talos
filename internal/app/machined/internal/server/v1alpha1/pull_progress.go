// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package runtime

import (
	"context"
	"errors"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/containerd/containerd/pkg/snapshotters"
	"github.com/containerd/containerd/v2/core/content"
	"github.com/containerd/containerd/v2/core/images"
	"github.com/containerd/containerd/v2/core/remotes"
	"github.com/containerd/containerd/v2/core/snapshots"
	"github.com/containerd/errdefs"
	"github.com/moby/moby/client/pkg/stringid"
	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"

	"github.com/siderolabs/talos/pkg/machinery/api/machine"
)

type pullProgress struct {
	srv         machine.MachineService_DebugContainerServer
	store       content.Store
	snapshotter snapshots.Snapshotter

	mu    sync.Mutex
	descs map[digest.Digest]ocispec.Descriptor // guarded by mu

	layers      []ocispec.Descriptor
	unpackStart map[digest.Digest]time.Time
}

func newPullProgress(srv machine.MachineService_DebugContainerServer, store content.Store, snapshotter snapshots.Snapshotter) *pullProgress {
	return &pullProgress{
		srv:         srv,
		store:       store,
		snapshotter: snapshotter,
		descs:       make(map[digest.Digest]ocispec.Descriptor),
		mu:          sync.Mutex{},
	}
}

func (p *pullProgress) showProgress(ctx context.Context) func() {
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

		if err := p.srv.Send(&machine.DebugContainerResponse{
			Resp: &machine.DebugContainerResponse_PullProgress{
				PullProgress: &machine.DebugContainerPullProgress{
					Id: "done",
				},
			},
		}); err != nil {
			log.Printf("debug container: failed to send pull progress: %s", err.Error())
		}
	}
}

func (p *pullProgress) trackProgress(ctx context.Context, start time.Time) error {
	err := p.trackOngoingPulls(ctx, start)
	if err != nil {
		return err
	}

	return p.trackPulledLayers(ctx)
}

func (p *pullProgress) trackOngoingPulls(ctx context.Context, start time.Time) error {
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

			p.sendUpdate(stringid.TruncateID(j.Digest.Encoded()),
				"Downloading",
				info.Offset,
				info.Total,
			)

			continue
		}

		info, err := p.store.Info(ctx, j.Digest)
		switch {
		case err != nil:
			if !errdefs.IsNotFound(err) {
				return err
			}

		case info.CreatedAt.After(start):
			p.sendUpdate(stringid.TruncateID(j.Digest.Encoded()),
				"Download complete",
				0, 0,
			)

			if images.IsLayerType(j.MediaType) {
				p.layers = append(p.layers, j)
			}

			p.remove(j)

		default:
			p.sendUpdate(stringid.TruncateID(j.Digest.Encoded()),
				"Already exists",
				0, 0,
			)

			if images.IsLayerType(j.MediaType) {
				p.layers = append(p.layers, j)
			}

			p.remove(j)
		}
	}

	return nil
}

func (p *pullProgress) trackPulledLayers(ctx context.Context) error {
	var committedIdx []int

	for idx, desc := range p.layers {
		walkFilter := "labels.\"" + snapshotters.TargetLayerDigestLabel + "\"==" + p.layers[idx].Digest.String()

		err := p.snapshotter.Walk(ctx, func(ctx context.Context, sn snapshots.Info) error {
			if sn.Kind == snapshots.KindActive {
				if p.unpackStart == nil {
					p.unpackStart = make(map[digest.Digest]time.Time)
				}

				var seconds int64

				if began, ok := p.unpackStart[desc.Digest]; !ok {
					p.unpackStart[desc.Digest] = time.Now()
				} else {
					seconds = int64(time.Since(began).Seconds())
				}

				p.sendUpdate(stringid.TruncateID(desc.Digest.Encoded()),
					"Extracting",
					1+seconds,
					0,
				)

				return nil
			}

			if sn.Kind == snapshots.KindCommitted {
				p.sendUpdate(stringid.TruncateID(desc.Digest.Encoded()),
					"Pull complete",
					0, 0,
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

	// Remove finished/committed layers from p.layers
	if len(committedIdx) > 0 {
		sort.Ints(committedIdx)

		for i := len(committedIdx) - 1; i >= 0; i-- {
			p.layers = append(p.layers[:committedIdx[i]], p.layers[committedIdx[i]+1:]...)
		}
	}

	return nil
}

func (p *pullProgress) sendUpdate(id, message string, current, total int64) {
	if err := p.srv.Send(&machine.DebugContainerResponse{
		Resp: &machine.DebugContainerResponse_PullProgress{
			PullProgress: &machine.DebugContainerPullProgress{
				Id:      id,
				Message: message,
				Current: current,
				Total:   total,
			},
		},
	}); err != nil {
		log.Printf("debug container: failed to send pull progress: %s", err.Error())
	}
}

func (p *pullProgress) add(desc ...ocispec.Descriptor) {
	p.mu.Lock()
	defer p.mu.Unlock()

	for _, d := range desc {
		if _, ok := p.descs[d.Digest]; ok {
			continue
		}

		p.descs[d.Digest] = d
	}
}

func (p *pullProgress) remove(desc ocispec.Descriptor) {
	p.mu.Lock()
	defer p.mu.Unlock()

	delete(p.descs, desc.Digest)
}

func (p *pullProgress) jobs() []ocispec.Descriptor {
	p.mu.Lock()
	defer p.mu.Unlock()

	descs := make([]ocispec.Descriptor, 0, len(p.descs))
	for _, d := range p.descs {
		descs = append(descs, d)
	}

	return descs
}
