// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package watch

import (
	"context"

	"golang.org/x/time/rate"
)

// RateLimitedTrigger wraps a Trigger with rate limiting.
type RateLimitedTrigger struct {
	trigger Trigger
	limiter *rate.Limiter
	ch      chan struct{}
}

// Interface check.
var _ Trigger = &RateLimitedTrigger{}

// NewRateLimitedTrigger creates a new RateLimitedTrigger with specified params.
//
// Trigger's goroutine exists when the context is canceled.
func NewRateLimitedTrigger(ctx context.Context, trigger Trigger, rateLimit rate.Limit, burst int) *RateLimitedTrigger {
	t := &RateLimitedTrigger{
		trigger: trigger,
		limiter: rate.NewLimiter(rateLimit, burst),
		ch:      make(chan struct{}),
	}

	go t.run(ctx)

	return t
}

// NewDefaultRateLimitedTrigger creates a new RateLimitedTrigger with default params.
func NewDefaultRateLimitedTrigger(ctx context.Context, trigger Trigger) *RateLimitedTrigger {
	const (
		defaultRate  = 10 // 10 events per second
		defaultBurst = 5  // 5 events
	)

	return NewRateLimitedTrigger(ctx, trigger, defaultRate, defaultBurst)
}

// QueueReconcile implements Trigger interface.
//
// The event is queued if the goroutine is ready to accept it (otherwise it's already
// busy processing a previous event).
// This function returns immediately.
func (t *RateLimitedTrigger) QueueReconcile() {
	select {
	case t.ch <- struct{}{}:
	default:
	}
}

func (t *RateLimitedTrigger) run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.ch:
		}

		if err := t.limiter.Wait(ctx); err != nil {
			return
		}

		t.trigger.QueueReconcile()
	}
}
