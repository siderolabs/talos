// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package upstream provides utilities for choosing upstream backends based on score.
package upstream

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Backend is an interface which should be implemented for a Pick entry.
type Backend interface {
	HealthCheck(ctx context.Context) error
}

type node struct {
	backend Backend
	score   float64
}

// ListOption allows to configure List.
type ListOption func(*List) error

// WithLowHighScores configures low and high score.
func WithLowHighScores(lowScore, highScore float64) ListOption {
	return func(l *List) error {
		if l.lowScore > 0 {
			return fmt.Errorf("lowScore should be non-positive")
		}

		if l.highScore < 0 {
			return fmt.Errorf("highScore should be non-positive")
		}

		if l.lowScore > l.highScore {
			return fmt.Errorf("lowScore should be less or equal to highScore")
		}

		l.lowScore, l.highScore = lowScore, highScore

		return nil
	}
}

// WithScoreDeltas configures fail and success score delta.
func WithScoreDeltas(failScoreDelta, successScoreDelta float64) ListOption {
	return func(l *List) error {
		if l.failScoreDelta >= 0 {
			return fmt.Errorf("failScoreDelta should be negative")
		}

		if l.successScoreDelta <= 0 {
			return fmt.Errorf("successScoreDelta should be positive")
		}

		l.failScoreDelta, l.successScoreDelta = failScoreDelta, successScoreDelta

		return nil
	}
}

// WithInitialScore configures initial backend score.
func WithInitialScore(initialScore float64) ListOption {
	return func(l *List) error {
		l.initialScore = initialScore

		return nil
	}
}

// WithHealthcheckInterval configures healthcheck interval.
func WithHealthcheckInterval(interval time.Duration) ListOption {
	return func(l *List) error {
		l.healthcheckInterval = interval

		return nil
	}
}

// WithHealthcheckTimeout configures healthcheck timeout (for each backend).
func WithHealthcheckTimeout(timeout time.Duration) ListOption {
	return func(l *List) error {
		l.healthcheckTimeout = timeout

		return nil
	}
}

// List of upstream Backends with healthchecks and different strategies to pick a node.
//
// List keeps track of Backends with score. Score is updated on health checks, and via external
// interface (e.g. when actual connection fails).
//
// Initial score is set via options (default is +1). Low and high scores defaults are (-3, +3).
// Backend score is limited by low and high scores. Each time healthcheck fails score is adjusted
// by fail delta score, and every successful check updates score by success score delta (defaults are -1/+1).
//
// Backend might be used if its score is not negative.
type List struct {
	lowScore, highScore               float64
	failScoreDelta, successScoreDelta float64
	initialScore                      float64

	healthcheckInterval time.Duration
	healthcheckTimeout  time.Duration

	healthWg        sync.WaitGroup
	healthCtx       context.Context
	healthCtxCancel context.CancelFunc

	// Following fields are protected by mutex
	mu sync.Mutex

	nodes   []node
	current int
}

// NewList initializes new list with upstream backends and options and starts health checks.
//
// List should be stopped with `.Shutdown()`.
func NewList(upstreams []Backend, options ...ListOption) (*List, error) {
	// initialize with defaults
	list := &List{
		lowScore:          -3.0,
		highScore:         3.0,
		failScoreDelta:    -1.0,
		successScoreDelta: 1.0,
		initialScore:      1.0,

		healthcheckInterval: 1 * time.Second,
		healthcheckTimeout:  100 * time.Millisecond,

		current: -1,
	}

	list.healthCtx, list.healthCtxCancel = context.WithCancel(context.Background())

	for _, opt := range options {
		if err := opt(list); err != nil {
			return nil, err
		}
	}

	list.nodes = make([]node, len(upstreams))

	for i := range list.nodes {
		list.nodes[i].backend = upstreams[i]
		list.nodes[i].score = list.initialScore
	}

	list.healthWg.Add(1)

	go list.healthcheck()

	return list, nil
}

// Shutdown stops healthchecks.
func (list *List) Shutdown() {
	list.healthCtxCancel()

	list.healthWg.Wait()
}

// Up increases backend score by success score delta.
func (list *List) Up(upstream Backend) {
	list.mu.Lock()
	defer list.mu.Unlock()

	for i := range list.nodes {
		if list.nodes[i].backend == upstream {
			list.nodes[i].score += list.successScoreDelta
			if list.nodes[i].score > list.highScore {
				list.nodes[i].score = list.highScore
			}
		}
	}
}

// Down decreases backend score by fail score delta.
func (list *List) Down(upstream Backend) {
	list.mu.Lock()
	defer list.mu.Unlock()

	for i := range list.nodes {
		if list.nodes[i].backend == upstream {
			list.nodes[i].score += list.failScoreDelta
			if list.nodes[i].score < list.lowScore {
				list.nodes[i].score = list.lowScore
			}
		}
	}
}

// Pick returns next backend to be used.
//
// Default policy is to pick healthy (non-negative score) backend in
// round-robin fashion.
func (list *List) Pick() (Backend, error) {
	list.mu.Lock()
	defer list.mu.Unlock()

	for j := 0; j < len(list.nodes); j++ {
		i := (list.current + 1 + j) % len(list.nodes)

		if list.nodes[i].score >= 0 {
			list.current = i

			return list.nodes[list.current].backend, nil
		}
	}

	return nil, fmt.Errorf("no upstreams available")
}

func (list *List) healthcheck() {
	defer list.healthWg.Done()

	ticker := time.NewTicker(list.healthcheckInterval)
	defer ticker.Stop()

	for {
		list.mu.Lock()
		nodes := append([]node(nil), list.nodes...)
		list.mu.Unlock()

		for _, node := range nodes {
			select {
			case <-list.healthCtx.Done():
				return
			default:
			}

			func() {
				ctx, ctxCancel := context.WithTimeout(list.healthCtx, list.healthcheckTimeout)
				defer ctxCancel()

				if err := node.backend.HealthCheck(ctx); err != nil {
					list.Down(node.backend)
				} else {
					list.Up(node.backend)
				}
			}()
		}

		select {
		case <-ticker.C:
		case <-list.healthCtx.Done():
			return
		}
	}
}
