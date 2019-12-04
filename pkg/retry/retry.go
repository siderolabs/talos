// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package retry

import (
	"fmt"
	"math/rand"
	"sync"
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

// ErrorSet represents a set of unique errors.
type ErrorSet struct {
	errs map[string]error

	mu sync.Mutex
}

func (e *ErrorSet) Error() string {
	if len(e.errs) == 0 {
		return ""
	}

	errString := fmt.Sprintf("%d error(s) occurred:", len(e.errs))
	for _, err := range e.errs {
		errString = fmt.Sprintf("%s\n%s", errString, err)
	}

	return errString
}

// Append adds the error to the set if the error is not already present.
func (e *ErrorSet) Append(err error) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.errs == nil {
		e.errs = make(map[string]error)
	}

	if _, ok := e.errs[err.Error()]; !ok {
		e.errs[err.Error()] = err
	}

	return e
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

	errs := &ErrorSet{}

	// We run the func first to avoid having to wait for the next tick.
	if err := f(); err != nil {
		if _, ok := err.(unexpectedError); ok {
			return errs.Append(err)
		}
	} else {
		return nil
	}

	for {
		select {
		case <-timer.C:
			return errs.Append(TimeoutError{})
		case <-t.StopChan():
			return nil
		case <-time.After(t.Tick()):
		}

		if err := f(); err != nil {
			switch err.(type) {
			case expectedError:
				// nolint: errcheck
				errs.Append(err)
				continue
			case unexpectedError:
				return errs.Append(err)
			}
		}

		return nil
	}
}
