// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package ctest

import (
	"fmt"
	"sort"
	"strings"

	"github.com/siderolabs/go-retry/retry"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type assertionAggregator struct {
	errors    map[string]struct{}
	failNow   bool
	hadErrors bool
}

func (agg *assertionAggregator) Errorf(format string, args ...any) {
	errorString := fmt.Sprintf(format, args...)

	if agg.errors == nil {
		agg.errors = make(map[string]struct{})
	}

	agg.errors[errorString] = struct{}{}
	agg.hadErrors = true
}

func (agg *assertionAggregator) FailNow() {
	agg.failNow = true
}

func (agg *assertionAggregator) Error() error {
	if !agg.hadErrors {
		return nil
	}

	lines := make([]string, 0, len(agg.errors))

	for errorString := range agg.errors {
		lines = append(lines, " * "+errorString)
	}

	sort.Strings(lines)

	return fmt.Errorf("%s", strings.Join(lines, "\n"))
}

// WrapRetry wraps the function with assertions and requires to return retry-compatible errors.
func WrapRetry(f func(*assert.Assertions, *require.Assertions)) func() error {
	return func() error {
		var errs assertionAggregator

		f(assert.New(&errs), require.New(&errs))

		if errs.failNow {
			return errs.Error()
		}

		return retry.ExpectedError(errs.Error())
	}
}
