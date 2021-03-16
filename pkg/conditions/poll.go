// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package conditions

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ErrSkipAssertion is used as a return value from AssertionFunc to indicate that this assertion
// (and, by extension, condition, and check) is to be skipped.
// It is not returned as an error by any Condition's Wait method
// but recorded as description and returned by String method.
var ErrSkipAssertion = errors.New("SKIP")

// AssertionFunc is called every poll interval until it returns nil.
type AssertionFunc func(ctx context.Context) error

type pollingCondition struct {
	lastErrMu  sync.Mutex
	lastErr    error
	lastErrSet bool

	assertion         AssertionFunc
	description       string
	timeout, interval time.Duration
}

func (p *pollingCondition) String() string {
	lastErr := "..."

	p.lastErrMu.Lock()
	if p.lastErrSet {
		if p.lastErr != nil {
			lastErr = p.lastErr.Error()
		} else {
			lastErr = OK
		}
	}
	p.lastErrMu.Unlock()

	return fmt.Sprintf("%s: %s", p.description, lastErr)
}

func (p *pollingCondition) Wait(ctx context.Context) error {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	timeoutCtx, timeoutCtxCancel := context.WithTimeout(ctx, p.timeout)
	defer timeoutCtxCancel()

	for {
		err := func() error {
			runCtx, runCtxCancel := context.WithTimeout(ctx, p.interval)
			defer runCtxCancel()

			err := p.assertion(runCtx)

			p.lastErrMu.Lock()
			p.lastErr = err
			p.lastErrSet = true
			p.lastErrMu.Unlock()

			return err
		}()

		if err == nil || err == ErrSkipAssertion {
			return nil
		}

		select {
		case <-timeoutCtx.Done():
			return timeoutCtx.Err()
		case <-ticker.C:
		}
	}
}

// PollingCondition converts AssertionFunc into Condition by calling it every interval until timeout
// is reached.
func PollingCondition(description string, assertion AssertionFunc, timeout, interval time.Duration) Condition {
	return &pollingCondition{
		assertion:   assertion,
		description: description,
		timeout:     timeout,
		interval:    interval,
	}
}
