// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package upstream_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"

	"github.com/talos-systems/talos/internal/pkg/loadbalancer/upstream"
	"github.com/talos-systems/talos/pkg/retry"
)

type mockBackend string

func (b mockBackend) HealthCheck(ctx context.Context) error {
	switch string(b) {
	case "fail":
		return errors.New("fail")
	case "success":
		return nil
	default:
		<-ctx.Done()

		return ctx.Err()
	}
}

type ListSuite struct {
	suite.Suite
}

func (suite *ListSuite) TestEmpty() {
	l, err := upstream.NewList(nil)
	suite.Require().NoError(err)

	defer l.Shutdown()

	backend, err := l.Pick()
	suite.Assert().Nil(backend)
	suite.Assert().EqualError(err, "no upstreams available")
}

func (suite *ListSuite) TestRoundRobin() {
	l, err := upstream.NewList([]upstream.Backend{mockBackend("one"), mockBackend("two"), mockBackend("three")})
	suite.Require().NoError(err)

	defer l.Shutdown()

	backend, err := l.Pick()
	suite.Assert().Equal(mockBackend("one"), backend)
	suite.Assert().NoError(err)

	backend, err = l.Pick()
	suite.Assert().Equal(mockBackend("two"), backend)
	suite.Assert().NoError(err)

	backend, err = l.Pick()
	suite.Assert().Equal(mockBackend("three"), backend)
	suite.Assert().NoError(err)

	backend, err = l.Pick()
	suite.Assert().Equal(mockBackend("one"), backend)
	suite.Assert().NoError(err)
}

func (suite *ListSuite) TestDownUp() {
	l, err := upstream.NewList(
		[]upstream.Backend{
			mockBackend("one"),
			mockBackend("two"),
			mockBackend("three"),
		},
		upstream.WithLowHighScores(-3, 3),
		upstream.WithInitialScore(1),
		upstream.WithScoreDeltas(-1, 1),
		upstream.WithHealthcheckInterval(time.Hour),
	)
	suite.Require().NoError(err)

	defer l.Shutdown()

	backend, err := l.Pick()
	suite.Assert().Equal(mockBackend("one"), backend)
	suite.Assert().NoError(err)

	l.Down(mockBackend("two"))   // score == 0
	l.Down(mockBackend("two"))   // score == -1
	l.Down(mockBackend("three")) // score == 0

	backend, err = l.Pick()
	suite.Assert().Equal(mockBackend("three"), backend)
	suite.Assert().NoError(err)

	backend, err = l.Pick()
	suite.Assert().Equal(mockBackend("one"), backend)
	suite.Assert().NoError(err)

	backend, err = l.Pick()
	suite.Assert().Equal(mockBackend("three"), backend)
	suite.Assert().NoError(err)

	l.Down(mockBackend("three")) // score == -1
	l.Up(mockBackend("two"))     // score == 0
	l.Up(mockBackend("two"))     // score == 1
	l.Up(mockBackend("two"))     // score == 2
	l.Up(mockBackend("two"))     // score == 3
	l.Up(mockBackend("two"))     // score == 3 (capped at highScore)

	backend, err = l.Pick()
	suite.Assert().Equal(mockBackend("one"), backend)
	suite.Assert().NoError(err)

	backend, err = l.Pick()
	suite.Assert().Equal(mockBackend("two"), backend)
	suite.Assert().NoError(err)

	backend, err = l.Pick()
	suite.Assert().Equal(mockBackend("one"), backend)
	suite.Assert().NoError(err)

	l.Down(mockBackend("two")) // score == 2
	l.Down(mockBackend("two")) // score == 1
	l.Down(mockBackend("two")) // score == 0
	l.Down(mockBackend("two")) // score == -1

	backend, err = l.Pick()
	suite.Assert().Equal(mockBackend("one"), backend)
	suite.Assert().NoError(err)

	l.Down(mockBackend("two")) // score == -2
	l.Down(mockBackend("two")) // score == -3
	l.Down(mockBackend("two")) // score == -3 (capped at lowScore)

	backend, err = l.Pick()
	suite.Assert().Equal(mockBackend("one"), backend)
	suite.Assert().NoError(err)

	l.Up(mockBackend("two")) // score == -2
	l.Up(mockBackend("two")) // score == -1
	l.Up(mockBackend("two")) // score == 0

	backend, err = l.Pick()
	suite.Assert().Equal(mockBackend("two"), backend)
	suite.Assert().NoError(err)
}

func (suite *ListSuite) TestHealthcheck() {
	l, err := upstream.NewList(
		[]upstream.Backend{
			mockBackend("success"),
			mockBackend("fail"),
			mockBackend("timeout"),
		},
		upstream.WithLowHighScores(-1, 1),
		upstream.WithInitialScore(1),
		upstream.WithScoreDeltas(-1, 1),
		upstream.WithHealthcheckInterval(10*time.Millisecond),
		upstream.WithHealthcheckTimeout(time.Millisecond),
	)
	suite.Require().NoError(err)

	defer l.Shutdown()

	time.Sleep(20 * time.Millisecond) // let healthchecks run

	// when health info converges, "success" should be the only backend left
	suite.Require().NoError(retry.Constant(time.Second, retry.WithUnits(time.Millisecond)).Retry(func() error {
		for i := 0; i < 10; i++ {
			backend, err := l.Pick()
			if err != nil {
				return retry.UnexpectedError(err)
			}

			if backend.(mockBackend) != mockBackend("success") {
				return retry.ExpectedError(fmt.Errorf("unexpected %v", backend))
			}
		}

		return nil
	}))
}

func TestListSuite(t *testing.T) {
	suite.Run(t, new(ListSuite))
}
