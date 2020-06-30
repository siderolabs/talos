// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package conditions

import (
	"context"
	"strings"
	"sync"

	"github.com/hashicorp/go-multierror"
)

type all struct {
	mu sync.Mutex

	conditions []Condition
}

type waitResult struct {
	i   int
	err error
}

func (a *all) Wait(ctx context.Context) error {
	errCh := make(chan waitResult)

	a.mu.Lock()
	for i := range a.conditions {
		go func(i int) {
			errCh <- waitResult{
				err: a.conditions[i].Wait(ctx),
				i:   i,
			}
		}(i)
	}
	a.mu.Unlock()

	err := (*multierror.Error)(nil)

	for range a.conditions {
		res := <-errCh

		a.mu.Lock()
		a.conditions[res.i] = nil
		a.mu.Unlock()

		err = multierror.Append(err, res.err)
	}

	// collapse errors if any of them is context canceled
	if err != nil {
		for _, e := range err.Errors {
			if e == context.Canceled {
				return e
			}
		}
	}

	return err.ErrorOrNil()
}

func (a *all) String() string {
	descriptions := []string(nil)

	a.mu.Lock()
	for _, c := range a.conditions {
		if c != nil {
			descriptions = append(descriptions, c.String())
		}
	}
	a.mu.Unlock()

	return strings.Join(descriptions, ", ")
}

// WaitForAll creates a condition which waits for all the conditions to be successful.
func WaitForAll(conditions ...Condition) Condition {
	res := &all{}

	for _, c := range conditions {
		if multi, ok := c.(*all); ok {
			// flatten lists
			res.conditions = append(res.conditions, multi.conditions...)
		} else {
			res.conditions = append(res.conditions, c)
		}
	}

	return res
}
