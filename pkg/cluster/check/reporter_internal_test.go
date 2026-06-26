// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package check

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/siderolabs/talos/pkg/conditions"
	"github.com/siderolabs/talos/pkg/reporter"
)

// statefulCondition is a fake condition that implements Stateful.
type statefulCondition struct {
	description string
	state       conditions.State
	lastErr     error
}

func (c statefulCondition) String() string { return c.description }

func (c statefulCondition) Wait(context.Context) error { return nil }

func (c statefulCondition) State() (conditions.State, error) { return c.state, c.lastErr }

// stringCondition is a non-Stateful fake (for testing the fallback path).
type stringCondition string

func (c stringCondition) String() string { return string(c) }

func (c stringCondition) Wait(context.Context) error { return nil }

func TestConditionToUpdate(t *testing.T) {
	for _, test := range []struct {
		name string
		cond conditions.Condition
		want reporter.Status
	}{
		{
			name: "stateful: running (unpolled)",
			cond: statefulCondition{description: "etcd to be healthy", state: conditions.StateRunning},
			want: reporter.StatusRunning,
		},
		{
			name: "stateful: running (transient error)",
			cond: statefulCondition{description: "etcd to be healthy", state: conditions.StateRunning, lastErr: context.Canceled},
			want: reporter.StatusRunning,
		},
		{
			name: "stateful: succeeded",
			cond: statefulCondition{description: "etcd to be healthy", state: conditions.StateSucceeded},
			want: reporter.StatusSucceeded,
		},
		{
			name: "stateful: skipped",
			cond: statefulCondition{description: "etcd to be healthy", state: conditions.StateSkipped},
			want: reporter.StatusSkip,
		},
		{
			name: "stateful: failed",
			cond: statefulCondition{description: "etcd to be healthy", state: conditions.StateFailed, lastErr: errors.New("connection refused")},
			want: reporter.StatusError,
		},
		{
			name: "non-Stateful fallback",
			cond: stringCondition("etcd to be healthy: OK"),
			want: reporter.StatusSucceeded,
		},
		{
			name: "non-Stateful failed",
			cond: stringCondition("etcd to be healthy"),
			want: reporter.StatusError,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			update := conditionToUpdate(test.cond)

			assert.Equal(t, test.want, update.Status)
		})
	}
}
