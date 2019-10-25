// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package retry

import (
	"math/rand"
	"time"
)

// RetryableFunc represents a function that can be retried.
type RetryableFunc func() error

// Retryer defines the requirements for retrying a function.
type Retryer interface {
	Retry(RetryableFunc) error
}

// Ticker defines the requirements for providing a clock to the retry logic.
type Ticker interface {
	Tick() time.Duration
	StopChan() <-chan struct{}
	Stop()
}

// TimeoutError represents a timeout error.
type TimeoutError struct{}

func (TimeoutError) Error() string {
	return "timeout"
}

// IsTimeout reutrns if the provided error is a timeout error.
func IsTimeout(err error) bool {
	_, ok := err.(TimeoutError)

	return ok
}

type expectedError struct{ error }

type unexpectedError struct{ error }

type retryer struct {
	duration time.Duration
	options  *Options
}

type ticker struct {
	C       chan time.Time
	options *Options
	rand    *rand.Rand
	s       chan struct{}
}

func (t ticker) Jitter() time.Duration {
	if int(t.options.Jitter) == 0 {
		return time.Duration(0)
	}

	if t.rand == nil {
		t.rand = rand.New(rand.NewSource(time.Now().UnixNano()))
	}

	return time.Duration(t.rand.Int63n(int64(t.options.Jitter)))
}

func (t ticker) StopChan() <-chan struct{} {
	return t.s
}

func (t ticker) Stop() {
	t.s <- struct{}{}
}

// ExpectedError error represents an error that is expected by the retrying
// function. This error is ignored.
func ExpectedError(err error) error {
	return expectedError{err}
}

// UnexpectedError error represents an error that is unexpected by the retrying
// function. This error is fatal.
func UnexpectedError(err error) error {
	return unexpectedError{err}
}

func retry(f RetryableFunc, d time.Duration, t Ticker) error {
	timer := time.NewTimer(d)
	defer timer.Stop()

	// We run the func first to avoid having to wait for the next tick.
	if err := f(); err != nil {
		if _, ok := err.(unexpectedError); ok {
			return err
		}
	} else {
		return nil
	}

	for {
		select {
		case <-timer.C:
			return TimeoutError{}
		case <-t.StopChan():
			return nil
		case <-time.After(t.Tick()):
		}

		if err := f(); err != nil {
			switch err.(type) {
			case expectedError:
				continue
			case unexpectedError:
				return err
			}
		}

		return nil
	}
}
