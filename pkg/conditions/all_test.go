// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package conditions_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/pkg/conditions"
)

type AllSuite struct {
	suite.Suite
}

type MockCondition struct {
	description string
	errCh       chan error
}

func (mc *MockCondition) String() string {
	return mc.description
}

func (mc *MockCondition) Wait(ctx context.Context) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-mc.errCh:
		return err
	}
}

func (suite *AllSuite) TestString() {
	suite.Require().Equal("A, B", conditions.WaitForAll(
		&MockCondition{description: "A"},
		&MockCondition{description: "B"},
	).String())

	suite.Require().Equal("A", conditions.WaitForAll(
		&MockCondition{description: "A"},
	).String())

	conds := []conditions.Condition{
		&MockCondition{description: "A", errCh: make(chan error)},
		&MockCondition{description: "B", errCh: make(chan error)},
	}

	waiter := conditions.WaitForAll(conds...)

	done := make(chan error)

	go func() {
		done <- waiter.Wait(context.Background())
	}()

	suite.Require().Equal("A, B", waiter.String())
	conds[0].(*MockCondition).errCh <- nil
	time.Sleep(50 * time.Millisecond)

	// done waiting for 'A', so description should now be shorter
	suite.Require().Equal("B", waiter.String())

	conds[1].(*MockCondition).errCh <- nil
	<-done
}

func (suite *AllSuite) TestFlatten() {
	conds1 := []conditions.Condition{
		&MockCondition{description: "A", errCh: make(chan error)},
		&MockCondition{description: "B", errCh: make(chan error)},
	}
	conds2 := []conditions.Condition{
		&MockCondition{description: "C", errCh: make(chan error)},
		&MockCondition{description: "D", errCh: make(chan error)},
	}

	waiter := conditions.WaitForAll(conditions.WaitForAll(conds1...), conditions.WaitForAll(conds2...))
	suite.Require().Equal("A, B, C, D", waiter.String())

	ctx, ctxCancel := context.WithCancel(context.Background())
	defer ctxCancel()

	done := make(chan error)

	go func() {
		done <- waiter.Wait(ctx)
	}()

	conds1[0].(*MockCondition).errCh <- nil
	conds2[1].(*MockCondition).errCh <- nil

	time.Sleep(50 * time.Millisecond)

	suite.Require().Equal("B, C", waiter.String())

	ctxCancel()

	suite.Require().Equal(context.Canceled, <-done)
}

func TestAllSuite(t *testing.T) {
	suite.Run(t, new(AllSuite))
}
